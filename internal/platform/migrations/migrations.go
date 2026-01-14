package migrations

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Run applies the schema for the bounded contexts. Intended to replace adapter-level automigrate.
func Run(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.AutoMigrate(
		&petRecord{},
		&orderRecord{},
		&userRecord{},
		&sessionRecord{},
	)
}

// Pet schema mirrors the pets Postgres adapter.
type petRecord struct {
	ID                 int64             `gorm:"primaryKey;column:id"`
	CategoryID         *int64            `gorm:"column:category_id"`
	CategoryName       string            `gorm:"column:category_name"`
	Name               string            `gorm:"column:name"`
	PhotoURLs          pq.StringArray    `gorm:"column:photo_urls;type:text[]"`
	Status             string            `gorm:"column:status;type:varchar(32);index"`
	HairLengthCm       float64           `gorm:"column:hair_length_cm"`
	TagIDs             pq.Int64Array     `gorm:"column:tag_ids;type:bigint[]"`
	TagNames           pq.StringArray    `gorm:"column:tag_names;type:text[]"`
	ExternalProvider   string            `gorm:"column:external_provider"`
	ExternalID         string            `gorm:"column:external_id"`
	ExternalAttributes map[string]string `gorm:"column:external_attributes;serializer:json"`
	CreatedAt          time.Time         `gorm:"column:created_at"`
	UpdatedAt          time.Time         `gorm:"column:updated_at"`
}

func (petRecord) TableName() string { return "pets" }

// Order schema mirrors the store Postgres adapter.
type orderRecord struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	PetID     int64     `gorm:"column:pet_id;index:idx_orders_status_pet"`
	Quantity  int32     `gorm:"column:quantity"`
	ShipDate  time.Time `gorm:"column:ship_date"`
	Status    string    `gorm:"column:status;type:varchar(32);index:idx_orders_status_pet"`
	Complete  bool      `gorm:"column:complete"`
	CreatedAt time.Time `gorm:"column:created_at;index"`
	UpdatedAt time.Time `gorm:"column:updated_at;index"`
}

func (orderRecord) TableName() string { return "orders" }

// User schema mirrors the users Postgres adapter.
type userRecord struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	Username  string    `gorm:"column:username;uniqueIndex"`
	FirstName string    `gorm:"column:first_name"`
	LastName  string    `gorm:"column:last_name"`
	Email     string    `gorm:"column:email"`
	Password  string    `gorm:"column:password_hash"`
	Phone     string    `gorm:"column:phone"`
	Status    int32     `gorm:"column:status"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (userRecord) TableName() string { return "users" }

// Session schema mirrors the session store.
type sessionRecord struct {
	Token     string     `gorm:"primaryKey;column:token;size:512"`
	Username  string     `gorm:"column:username;index"`
	ExpiresAt *time.Time `gorm:"column:expires_at;index"`
	CreatedAt time.Time  `gorm:"column:created_at;index"`
	UpdatedAt time.Time  `gorm:"column:updated_at;index"`
}

func (sessionRecord) TableName() string { return "user_sessions" }
