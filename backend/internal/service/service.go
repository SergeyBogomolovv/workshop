// Пакет service отвечает за валидацию, проверку доступа и правила обработки ремонта.
package service

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"workshop/internal/domain"
	"workshop/internal/security"
)

// Repository задаёт границу хранилища, используемого сервисами приложения.
type Repository interface {
	CreateUser(context.Context, domain.User) error
	UserByLogin(context.Context, string) (domain.User, error)
	UserByID(context.Context, string) (domain.User, error)
	UpdateProfile(context.Context, domain.User) error
	CreateSession(context.Context, domain.Session) error
	Session(context.Context, string) (domain.Session, error)
	RevokeSession(context.Context, string) error
	Catalog(context.Context) (domain.Catalog, error)
	Occupied(context.Context, time.Time, time.Time) ([]time.Time, error)
	CreateAppointment(context.Context, domain.Appointment) error
	Appointments(context.Context, string, string, int, int) ([]domain.Appointment, error)
	Appointment(context.Context, string) (domain.Appointment, error)
	History(context.Context, string) ([]domain.History, error)
	Transition(context.Context, string, string, string, string) error
}

// Service объединяет бизнес-правила и защиту персональных данных.
type Service struct {
	repo  Repository
	vault *security.Vault
	dummy string
	zone  *time.Location
	now   func() time.Time
}

// New настраивает защиту от анализа времени проверки пароля и часовой пояс мастерской.
func New(repo Repository, vault *security.Vault) (*Service, error) {
	random, err := security.RandomToken()
	if err != nil {
		return nil, err
	}

	dummy, err := security.HashPassword(random)
	if err != nil {
		return nil, err
	}

	zone, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return nil, err
	}

	return &Service{
		repo:  repo,
		vault: vault,
		dummy: dummy,
		zone:  zone,
		now:   time.Now,
	}, nil
}

// Registration описывает допустимые поля публичной регистрации.
type Registration struct {
	Login         string `json:"login"`
	Password      string `json:"password"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Consent       bool   `json:"consent"`
	PolicyVersion string `json:"policy_version"`
}

// Profile содержит редактируемые контактные данные.
type Profile struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// Booking описывает выбранную услугу и будущий интервал приёма устройства.
type Booking struct {
	ServiceID   string    `json:"service_id"`
	Category    string    `json:"category"`
	SlotStart   time.Time `json:"slot_start"`
	Description string    `json:"description"`
}

var loginPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{3,31}$`)
var phonePattern = regexp.MustCompile(`^\+7[0-9]{10}$`)
var namePattern = regexp.MustCompile(`^[\p{L} '\-]+$`)

// invalid создаёт безопасное сообщение об ошибке валидации.
func invalid(message string) error {
	return domain.Fail(400, "validation", message)
}

// validLength считает символы Unicode, а не байты.
func validLength(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)

	return utf8.ValidString(s) && n >= min && n <= max
}

// validUUID принимает только каноническое представление UUID, используемое API.
func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)

	return err == nil && parsed.String() == value
}

// ValidateProfile нормализует и проверяет все обязательные контактные поля.
func ValidateProfile(p *Profile) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Email = strings.TrimSpace(p.Email)
	if !validLength(p.Name, 2, 80) || !namePattern.MatchString(p.Name) ||
		strings.Trim(p.Name, " '-") == "" {
		return invalid("Имя: от 2 до 80 символов, только буквы, пробел, дефис и апостроф")
	}

	if !phonePattern.MatchString(p.Phone) {
		return invalid("Введите телефон в формате +79991234567")
	}

	address, err := mail.ParseAddress(p.Email)
	if err != nil || address.Address != p.Email || address.Name != "" || len(p.Email) > 254 ||
		!strings.Contains(p.Email, ".") {
		return invalid("Введите корректный email")
	}

	return nil
}

// Register создаёт учётную запись с ролью, явно заданной сервером.
func (s *Service) Register(ctx context.Context, r Registration, role string) error {
	if !loginPattern.MatchString(r.Login) {
		return invalid("Логин: 4–32 латинские буквы, цифры или _, первый символ — буква")
	}

	if !validLength(r.Password, 15, 128) {
		return invalid("Пароль должен содержать от 15 до 128 символов")
	}

	if !r.Consent || r.PolicyVersion != domain.PolicyVersion {
		return invalid("Необходимо согласие с текущей политикой обработки данных")
	}

	p := Profile{
		r.Name,
		r.Phone,
		r.Email,
	}
	if err := ValidateProfile(&p); err != nil {
		return err
	}

	if role != "client" && role != "admin" {
		return invalid("Неизвестная роль")
	}

	index := s.vault.LoginIndex(r.Login)
	_, err := s.repo.UserByLogin(ctx, index)
	if err == nil {
		return domain.Fail(409, "login_taken", "Этот логин уже занят")
	}

	if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	hash, err := security.HashPassword(r.Password)
	if err != nil {
		return err
	}

	user := domain.User{
		ID:            uuid.NewString(),
		Login:         strings.ToLower(r.Login),
		LoginIndex:    index,
		PasswordHash:  hash,
		Name:          p.Name,
		Phone:         p.Phone,
		Email:         p.Email,
		Role:          role,
		PolicyVersion: r.PolicyVersion,
	}
	if err = s.encryptUser(&user); err != nil {
		return err
	}

	return s.repo.CreateUser(ctx, user)
}

// Login проверяет учётные данные и сохраняет новую отзываемую сессию.
func (s *Service) Login(ctx context.Context, login, password string) (string, domain.User, error) {
	denied := domain.Fail(401, "invalid_credentials", "Логин или пароль некорректен")
	if !validLength(password, 1, 128) || !loginPattern.MatchString(login) {
		security.VerifyPassword(s.dummy, "invalid")

		return "", domain.User{}, denied
	}

	user, err := s.repo.UserByLogin(ctx, s.vault.LoginIndex(login))
	if errors.Is(err, domain.ErrNotFound) {
		security.VerifyPassword(s.dummy, password)

		return "", domain.User{}, denied
	}

	if err != nil {
		return "", user, err
	}

	if !security.VerifyPassword(user.PasswordHash, password) {
		return "", domain.User{}, denied
	}

	token, err := security.RandomToken()
	if err != nil {
		return "", user, err
	}

	csrf, err := security.RandomToken()
	if err != nil {
		return "", user, err
	}

	session := domain.Session{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: security.TokenHash(token),
		CSRF:      csrf,
		ExpiresAt: s.now().Add(24 * time.Hour),
	}
	if err = s.repo.CreateSession(ctx, session); err != nil {
		return "", user, err
	}

	err = s.decryptUser(&user)

	return token, user, err
}

// LoginKey возвращает индекс на основе секретного ключа для ограничения попыток входа.
func (s *Service) LoginKey(login string) string {
	return s.vault.LoginIndex(login)
}

// Authenticate проверяет срок сессии и получает текущую роль пользователя.
func (s *Service) Authenticate(
	ctx context.Context,
	token string,
) (domain.Session, domain.User, error) {
	if len(token) != 43 {
		return domain.Session{}, domain.User{}, domain.ErrNotFound
	}

	session, err := s.repo.Session(ctx, security.TokenHash(token))
	if err != nil {
		return session, domain.User{}, err
	}

	if !session.ExpiresAt.After(s.now()) {
		return session, domain.User{}, domain.ErrNotFound
	}

	user, err := s.repo.UserByID(ctx, session.UserID)
	if err == nil {
		err = s.decryptUser(&user)
	}

	return session, user, err
}

// Logout отзывает текущую сессию в хранилище.
func (s *Service) Logout(
	ctx context.Context,
	id string,
) error {
	return s.repo.RevokeSession(ctx, id)
}

// UpdateProfile проверяет и шифрует новые контактные данные.
func (s *Service) UpdateProfile(
	ctx context.Context,
	user domain.User,
	p Profile,
) (domain.User, error) {
	if err := ValidateProfile(&p); err != nil {
		return user, err
	}

	user.Name = p.Name
	user.Phone = p.Phone
	user.Email = p.Email
	encrypted := user
	if err := s.encryptUser(&encrypted); err != nil {
		return user, err
	}

	return user, s.repo.UpdateProfile(ctx, encrypted)
}

// encryptUser связывает каждое закрытое поле с идентификатором учётной записи.
func (s *Service) encryptUser(user *domain.User) error {
	for field, value := range map[string]*string{
		"login": &user.Login,
		"name":  &user.Name,
		"phone": &user.Phone,
		"email": &user.Email,
	} {
		encrypted, err := s.vault.Encrypt(*value, user.ID+":"+field)
		if err != nil {
			return err
		}

		*value = encrypted
	}

	return nil
}

// decryptUser восстанавливает контактные данные только в рамках разрешённой операции.
func (s *Service) decryptUser(user *domain.User) error {
	for field, value := range map[string]*string{
		"login": &user.Login,
		"name":  &user.Name,
		"phone": &user.Phone,
		"email": &user.Email,
	} {
		plain, err := s.vault.Decrypt(*value, user.ID+":"+field)
		if err != nil {
			return err
		}

		*value = plain
	}

	return nil
}

// Catalog возвращает общедоступный каталог услуг.
func (s *Service) Catalog(
	ctx context.Context,
) (domain.Catalog, error) {
	return s.repo.Catalog(ctx)
}

// validSlot проверяет горизонт записи по Москве и точные часовые границы интервалов.
func (s *Service) validSlot(slot time.Time) bool {
	now := s.now().In(s.zone)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.zone)
	local := slot.In(s.zone)

	return slot.After(now) && !slot.Before(start) && slot.Before(start.AddDate(0, 0, 30)) &&
		local.Hour() >= 10 &&
		local.Hour() <= 17 &&
		local.Minute() == 0 &&
		local.Second() == 0 &&
		local.Nanosecond() == 0
}

// Slots вычисляет будущие свободные интервалы приёма для допустимой календарной даты.
func (s *Service) Slots(ctx context.Context, date string) ([]time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", date, s.zone)
	if err != nil {
		return nil, invalid("Некорректная дата")
	}

	now := s.now().In(s.zone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.zone)
	if day.Before(today) || !day.Before(today.AddDate(0, 0, 30)) {
		return nil, invalid("Выберите дату в ближайшие 30 дней")
	}

	occupied, err := s.repo.Occupied(ctx, day, day.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}

	taken := map[int64]bool{}
	for _, t := range occupied {
		taken[t.Unix()] = true
	}

	slots := []time.Time{}
	for hour := 10; hour < 18; hour++ {
		t := day.Add(time.Duration(hour) * time.Hour)
		if s.validSlot(t) && !taken[t.Unix()] {
			slots = append(slots, t)
		}
	}

	return slots, nil
}

// Book проверяет совместимость услуги с устройством и шифрует новую заявку.
func (s *Service) Book(ctx context.Context, user domain.User, b Booking) (string, error) {
	if user.Role != "client" {
		return "", domain.Fail(403, "forbidden", "Запись доступна клиентам")
	}

	b.Description = strings.TrimSpace(b.Description)
	if !validLength(b.Description, 10, 1000) || strings.ContainsAny(b.Description, "<>") {
		return "", invalid("Описание: 10–1000 символов без HTML-разметки")
	}

	if !validUUID(b.ServiceID) {
		return "", invalid("Выберите услугу")
	}

	if !s.validSlot(b.SlotStart) {
		return "", invalid("Выберите свободное время в ближайшие 30 дней")
	}

	catalog, err := s.repo.Catalog(ctx)
	if err != nil {
		return "", err
	}

	compatible := false
	for _, o := range catalog.Services {
		if o.ID == b.ServiceID {
			for _, c := range o.Categories {
				if c == b.Category {
					compatible = true
				}
			}
		}
	}

	if !compatible {
		return "", invalid("Услуга недоступна для выбранного устройства")
	}

	id := uuid.NewString()
	description, err := s.vault.Encrypt(b.Description, id+":description")
	if err != nil {
		return "", err
	}

	a := domain.Appointment{
		ID:          id,
		UserID:      user.ID,
		ServiceID:   b.ServiceID,
		Category:    b.Category,
		SlotStart:   b.SlotStart,
		Description: description,
		Status:      "new",
	}

	return id, s.repo.CreateAppointment(ctx, a)
}

// List возвращает ограниченную страницу заявок в пределах доступа владельца или администратора.
func (s *Service) List(
	ctx context.Context,
	user domain.User,
	status string,
	page int,
) (domain.Page, error) {
	if page < 1 || page > 100000 {
		return domain.Page{}, invalid("Некорректный номер страницы")
	}

	if status != "" && !KnownStatus(status) {
		return domain.Page{}, invalid("Неизвестный статус")
	}

	owner := user.ID
	if user.Role == "admin" {
		owner = ""
	}

	items, err := s.repo.Appointments(ctx, owner, status, 21, (page-1)*20)
	if err != nil {
		return domain.Page{}, err
	}

	more := len(items) > 20
	if more {
		items = items[:20]
	}

	for i := range items {
		items[i].Description = ""
	}

	return domain.Page{
		Items:   items,
		Page:    page,
		HasMore: more,
	}, nil
}

// Detail проверяет владельца перед расшифровкой заявки или раскрытием контактов.
func (s *Service) Detail(ctx context.Context, user domain.User, id string) (domain.Detail, error) {
	if !validUUID(id) {
		return domain.Detail{}, invalid("Некорректный идентификатор")
	}

	a, err := s.repo.Appointment(ctx, id)
	if err != nil {
		return domain.Detail{}, err
	}

	if user.Role != "admin" && a.UserID != user.ID {
		return domain.Detail{}, domain.ErrNotFound
	}

	a.Description, err = s.vault.Decrypt(a.Description, id+":description")
	if err != nil {
		return domain.Detail{}, err
	}

	history, err := s.repo.History(ctx, id)
	if err != nil {
		return domain.Detail{}, err
	}

	d := domain.Detail{Appointment: a, History: history}
	if user.Role == "admin" {
		client, err := s.repo.UserByID(ctx, a.UserID)
		if err != nil {
			return d, err
		}

		if err = s.decryptUser(&client); err != nil {
			return d, err
		}

		d.Client = &client
	}

	return d, nil
}

// KnownStatus проверяет принадлежность статуса допустимому набору значений.
func KnownStatus(status string) bool {
	switch status {
	case "new", "accepted", "in_progress", "ready", "completed", "cancelled":
		return true
	}

	return false
}

// CanTransition определяет допустимые переходы ремонта и условия отмены.
func CanTransition(from, to string, admin bool) bool {
	if to == "cancelled" && (from == "new" || from == "accepted") {
		return true
	}

	if !admin {
		return false
	}

	next := map[string]string{
		"new":         "accepted",
		"accepted":    "in_progress",
		"in_progress": "ready",
		"ready":       "completed",
	}
	expected, ok := next[from]

	return ok && expected == to
}

// ChangeStatus разрешает переход статуса и атомарно сохраняет его вместе с историей.
func (s *Service) ChangeStatus(ctx context.Context, user domain.User, id, to string) error {
	d, err := s.Detail(ctx, user, id)
	if err != nil {
		return err
	}

	if !CanTransition(d.Appointment.Status, to, user.Role == "admin") {
		return domain.Fail(409, "invalid_transition", "Этот переход статуса недоступен")
	}

	return s.repo.Transition(ctx, id, d.Appointment.Status, to, user.ID)
}
