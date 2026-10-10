// Пакет postgres реализует репозитории приложения с параметризованными SQL-запросами.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"workshop/internal/domain"
)

var builder = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

const userColumns = "id, login_cipher, login_index, password_hash, " +
	"name_cipher, phone_cipher, email_cipher, role, policy_version"

// Repository управляет пулом соединений и повторно используемыми подготовленными запросами.
type Repository struct {
	db      *sqlx.DB
	byLogin *sqlx.Stmt
	byID    *sqlx.Stmt
	session *sqlx.Stmt
}

// Open подключается к БД и подготавливает частые запросы, освобождая ресурсы при ошибке.
func Open(ctx context.Context, dsn string) (*Repository, error) {
	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	r := &Repository{db: db}
	r.byLogin, err = db.PreparexContext(
		ctx,
		"SELECT "+userColumns+" FROM users WHERE login_index=$1",
	)
	if err != nil {
		r.Close()

		return nil, err
	}

	r.byID, err = db.PreparexContext(ctx, "SELECT "+userColumns+" FROM users WHERE id=$1")
	if err != nil {
		r.Close()

		return nil, err
	}

	r.session, err = db.PreparexContext(
		ctx,
		`SELECT id, user_id, token_hash, csrf_token, expires_at
		 FROM sessions
		 WHERE token_hash = $1 AND NOT revoked AND expires_at > now()`,
	)
	if err != nil {
		r.Close()

		return nil, err
	}

	return r, nil
}

// Close закрывает подготовленные запросы и пул соединений.
func (r *Repository) Close() {
	for _, s := range []*sqlx.Stmt{
		r.byLogin,
		r.byID,
		r.session,
	} {
		if s != nil {
			_ = s.Close()
		}
	}

	_ = r.db.Close()
}

// translate преобразует ожидаемые ошибки ограничений PostgreSQL в безопасные ошибки приложения.
func translate(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}

	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		switch pg.ConstraintName {
		case "users_login_index_key":
			return domain.Fail(409, "login_taken", "Этот логин уже занят")
		case "appointments_slot_unique":
			return domain.Fail(409, "slot_taken", "Это время уже занято. Выберите другой слот.")
		}

		return domain.ErrConflict
	}

	return err
}

// execute строит SQL с отдельными параметрами и выполняет его в пуле или транзакции.
func execute(ctx context.Context, db sqlx.ExecerContext, query sq.Sqlizer) error {
	sqlText, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, sqlText, args...)

	return translate(err)
}

// CreateUser сохраняет зашифрованные данные пользователя с контролем уникальности в БД.
func (r *Repository) CreateUser(ctx context.Context, u domain.User) error {
	query := builder.Insert("users").
		Columns(
			"id",
			"login_cipher",
			"login_index",
			"password_hash",
			"name_cipher",
			"phone_cipher",
			"email_cipher",
			"role",
			"policy_version",
		).
		Values(
			u.ID,
			u.Login,
			u.LoginIndex,
			u.PasswordHash,
			u.Name,
			u.Phone,
			u.Email,
			u.Role,
			u.PolicyVersion,
		)

	return execute(ctx, r.db, query)
}

// UserByLogin выполняет подготовленный поиск по защищённому индексу логина.
func (r *Repository) UserByLogin(ctx context.Context, index string) (domain.User, error) {
	var u domain.User
	err := r.byLogin.GetContext(ctx, &u, index)

	return u, translate(err)
}

// UserByID выполняет подготовленный поиск учётной записи.
func (r *Repository) UserByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.byID.GetContext(ctx, &u, id)

	return u, translate(err)
}

// UpdateProfile обновляет только редактируемые зашифрованные контактные поля.
func (r *Repository) UpdateProfile(ctx context.Context, u domain.User) error {
	return execute(
		ctx,
		r.db,
		builder.Update("users").
			Set("name_cipher", u.Name).
			Set("phone_cipher", u.Phone).
			Set("email_cipher", u.Email).
			Where(sq.Eq{"id": u.ID}),
	)
}

// CreateSession сохраняет хеш токена и секрет CSRF.
func (r *Repository) CreateSession(ctx context.Context, s domain.Session) error {
	return execute(
		ctx,
		r.db,
		builder.Insert("sessions").
			Columns("id", "user_id", "token_hash", "csrf_token", "expires_at").
			Values(s.ID, s.UserID, s.TokenHash, s.CSRF, s.ExpiresAt),
	)
}

// Session исключает отозванные и истёкшие сессии при выполнении подготовленного запроса.
func (r *Repository) Session(ctx context.Context, hash string) (domain.Session, error) {
	var s domain.Session
	err := r.session.GetContext(ctx, &s, hash)

	return s, translate(err)
}

// RevokeSession немедленно отзывает токен.
func (r *Repository) RevokeSession(ctx context.Context, id string) error {
	return execute(
		ctx,
		r.db,
		builder.Update("sessions").Set("revoked", true).Where(sq.Eq{"id": id}),
	)
}

// Catalog загружает активные услуги и допустимые категории устройств.
func (r *Repository) Catalog(ctx context.Context) (domain.Catalog, error) {
	const categoriesQuery = `
		SELECT code, name
		FROM device_categories
		ORDER BY code
	`
	const servicesQuery = `
		SELECT id, name, description, price_kopecks
		FROM services
		WHERE active
		ORDER BY price_kopecks, id
	`
	const compatibilityQuery = `
		SELECT category_code
		FROM service_categories
		WHERE service_id = $1
		ORDER BY category_code
	`

	catalog := domain.Catalog{
		Categories: []domain.Category{},
		Services:   []domain.Offering{},
	}
	if err := r.db.SelectContext(ctx, &catalog.Categories, categoriesQuery); err != nil {
		return catalog, err
	}

	if err := r.db.SelectContext(ctx, &catalog.Services, servicesQuery); err != nil {
		return catalog, err
	}

	for i := range catalog.Services {
		service := &catalog.Services[i]
		service.Categories = []string{}
		if err := r.db.SelectContext(ctx, &service.Categories, compatibilityQuery, service.ID); err != nil {
			return catalog, err
		}
	}

	return catalog, nil
}

// Occupied возвращает время начала неотменённых заявок в заданном календарном интервале.
func (r *Repository) Occupied(ctx context.Context, start, end time.Time) ([]time.Time, error) {
	slots := []time.Time{}
	err := r.db.SelectContext(
		ctx,
		&slots,
		`SELECT slot_start
		 FROM appointments
		 WHERE slot_start >= $1 AND slot_start < $2 AND status <> 'cancelled'`,
		start,
		end,
	)

	return slots, err
}

// CreateAppointment сохраняет заявку и первую запись истории в одной транзакции.
func (r *Repository) CreateAppointment(ctx context.Context, a domain.Appointment) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()
	err = execute(
		ctx,
		tx,
		builder.Insert("appointments").
			Columns("id", "user_id", "service_id", "category_code", "slot_start", "description_cipher", "status").
			Values(a.ID, a.UserID, a.ServiceID, a.Category, a.SlotStart, a.Description, a.Status),
	)
	if err != nil {
		return err
	}

	err = execute(
		ctx,
		tx,
		builder.Insert("appointment_status_history").
			Columns("appointment_id", "previous_status", "status", "actor_id").
			Values(a.ID, "", a.Status, a.UserID),
	)
	if err != nil {
		return err
	}

	return translate(tx.Commit())
}

// appointmentQuery задаёт общий набор полей с фиксированными SQL-идентификаторами.
func appointmentQuery() sq.SelectBuilder {
	return builder.Select(
		"a.id",
		"a.user_id",
		"a.service_id",
		"a.category_code",
		"a.slot_start",
		"a.description_cipher",
		"a.status",
		"a.created_at",
		"a.updated_at",
		"s.name AS service_name",
		"c.name AS category_name",
	).
		From("appointments a").
		Join("services s ON s.id=a.service_id").
		Join("device_categories c ON c.code=a.category_code")
}

// Appointments читает страницу заявок с необязательной фильтрацией по владельцу и статусу.
func (r *Repository) Appointments(
	ctx context.Context,
	owner, status string,
	limit, offset int,
) ([]domain.Appointment, error) {
	q := appointmentQuery().OrderBy("a.created_at DESC", "a.id DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset))
	if owner != "" {
		q = q.Where(sq.Eq{"a.user_id": owner})
	}

	if status != "" {
		q = q.Where(sq.Eq{"a.status": status})
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	items := []domain.Appointment{}
	err = r.db.SelectContext(ctx, &items, query, args...)

	return items, translate(err)
}

// Appointment получает заявку для проверки владельца на уровне сервиса.
func (r *Repository) Appointment(ctx context.Context, id string) (domain.Appointment, error) {
	var a domain.Appointment
	query, args, err := appointmentQuery().Where(sq.Eq{"a.id": id}).ToSql()
	if err != nil {
		return a, err
	}

	err = r.db.GetContext(ctx, &a, query, args...)

	return a, translate(err)
}

// History возвращает изменения статуса заявки в стабильном хронологическом порядке.
func (r *Repository) History(ctx context.Context, id string) ([]domain.History, error) {
	items := []domain.History{}
	err := r.db.SelectContext(
		ctx,
		&items,
		`SELECT previous_status, status, created_at
		 FROM appointment_status_history
		 WHERE appointment_id = $1
		 ORDER BY id`,
		id,
	)

	return items, err
}

// Transition изменяет ожидаемый предыдущий статус и добавляет историю в одной транзакции.
func (r *Repository) Transition(ctx context.Context, id, from, to, actor string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()
	result, err := tx.ExecContext(
		ctx,
		"UPDATE appointments SET status=$1,updated_at=now() WHERE id=$2 AND status=$3",
		to,
		id,
		from,
	)
	if err != nil {
		return translate(err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if n != 1 {
		return domain.ErrConflict
	}

	err = execute(
		ctx,
		tx,
		builder.Insert("appointment_status_history").
			Columns("appointment_id", "previous_status", "status", "actor_id").
			Values(id, from, to, actor),
	)
	if err != nil {
		return err
	}

	return translate(tx.Commit())
}
