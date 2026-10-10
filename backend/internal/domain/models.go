// Пакет domain описывает данные приложения независимо от HTTP и хранилища.
package domain

import "time"

const PolicyVersion = "2026-10-10"

// User объединяет закрытые поля хранения и открытое представление профиля.
type User struct {
	ID            string `db:"id" json:"id"`
	Login         string `db:"login_cipher" json:"login"`
	LoginIndex    string `db:"login_index" json:"-"`
	PasswordHash  string `db:"password_hash" json:"-"`
	Name          string `db:"name_cipher" json:"name"`
	Phone         string `db:"phone_cipher" json:"phone"`
	Email         string `db:"email_cipher" json:"email"`
	Role          string `db:"role" json:"role"`
	PolicyVersion string `db:"policy_version" json:"-"`
}

// Session описывает отзываемую сессию без хранения исходного токена.
type Session struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	CSRF      string    `db:"csrf_token"`
	ExpiresAt time.Time `db:"expires_at"`
}

// Category обозначает поддерживаемый тип устройства.
type Category struct {
	Code string `db:"code" json:"code"`
	Name string `db:"name" json:"name"`
}

// Offering описывает услугу ремонта и совместимые устройства.
type Offering struct {
	ID          string   `db:"id" json:"id"`
	Name        string   `db:"name" json:"name"`
	Description string   `db:"description" json:"description"`
	Price       int      `db:"price_kopecks" json:"price_kopecks"`
	Categories  []string `json:"categories"`
}

// Catalog содержит общедоступный каталог услуг ремонта.
type Catalog struct {
	Categories []Category `json:"categories"`
	Services   []Offering `json:"services"`
}

// Appointment содержит заявку на ремонт и названия для отображения.
type Appointment struct {
	ID           string    `db:"id" json:"id"`
	UserID       string    `db:"user_id" json:"-"`
	ServiceID    string    `db:"service_id" json:"service_id"`
	Category     string    `db:"category_code" json:"category"`
	SlotStart    time.Time `db:"slot_start" json:"slot_start"`
	Description  string    `db:"description_cipher" json:"description"`
	Status       string    `db:"status" json:"status"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
	ServiceName  string    `db:"service_name" json:"service_name"`
	CategoryName string    `db:"category_name" json:"category_name"`
}

// History описывает атомарное изменение статуса.
type History struct {
	Previous  string    `db:"previous_status" json:"previous_status"`
	Status    string    `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Detail дополняет заявку историей и контактами, доступными только администратору.
type Detail struct {
	Appointment Appointment `json:"appointment"`
	History     []History   `json:"history"`
	Client      *User       `json:"client,omitempty"`
}

// Page содержит одну страницу списка заявок ограниченного размера.
type Page struct {
	Items   []Appointment `json:"items"`
	Page    int           `json:"page"`
	HasMore bool          `json:"has_more"`
}

// Error описывает ошибку приложения с безопасным сообщением для клиента.
type Error struct {
	Status  int
	Code    string
	Message string
}

// Error реализует стандартный интерфейс ошибок.
func (e *Error) Error() string {
	return e.Message
}

// Fail создаёт ошибку для клиента без раскрытия внутренних деталей хранилища.
func Fail(status int, code, message string) error {
	return &Error{
		status,
		code,
		message,
	}
}

var ErrNotFound = Fail(404, "not_found", "Запись не найдена")

var ErrConflict = Fail(
	409,
	"conflict",
	"Данные уже изменились. Обновите страницу и повторите действие.",
)
