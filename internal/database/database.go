package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"golang.org/x/oauth2"

	"duitno/internal/oauth"
)

type DB struct {
	*sql.DB
}

func getDSN() string {
	host := getEnv("DB_HOST", "127.0.0.1")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "duitno")
	port := getEnv("DB_PORT", "5432")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func EnsureDatabase() error {
	host := getEnv("DB_HOST", "127.0.0.1")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "duitno")
	port := getEnv("DB_PORT", "5432")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=disable", user, password, host, port)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var exists bool
	err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbname).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check database: %w", err)
	}

	if !exists {
		_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", dbname))
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		fmt.Printf("Database %s created successfully\n", dbname)
	}

	return nil
}

func Connect() (*DB, error) {
	dsn := getDSN()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

func RunMigrations(db *sql.DB) error {
	moduleRoot, err := findModuleRoot()
	if err != nil {
		return fmt.Errorf("failed to find module root: %w", err)
	}

	migrationsPath := filepath.Join(moduleRoot, "migrations")

	m, err := migrate.New(
		"file://"+migrationsPath,
		getDSN(),
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func (db *DB) FindOrCreateUser(provider oauth.Provider, userInfo *oauth.UserInfo, token *oauth2.Token) (string, error) {
	var userID string
	var exists bool

	err := db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)
	`, userInfo.Email).Scan(&exists)

	if err != nil {
		return "", err
	}

	if !exists {
		err = db.QueryRow(`
			INSERT INTO users (email, created_at, updated_at)
			VALUES ($1, $2, $2)
			RETURNING id
		`, userInfo.Email, time.Now()).Scan(&userID)
		if err != nil {
			return "", err
		}

		_, err = db.Exec(`
			INSERT INTO user_infos (user_id, full_name, created_at, updated_at)
			VALUES ($1, $2, $3, $3)
		`, userID, userInfo.Name, time.Now())
		if err != nil {
			return "", err
		}
	} else {
		err = db.QueryRow(`SELECT id FROM users WHERE email = $1`, userInfo.Email).Scan(&userID)
		if err != nil {
			return "", err
		}

		_, err = db.Exec(`
			UPDATE user_infos SET full_name = $1, updated_at = $2 WHERE user_id = $3
		`, userInfo.Name, time.Now(), userID)
		if err != nil {
			return "", err
		}
	}

	_, err = db.Exec(`
		INSERT INTO oauth_accounts (user_id, provider, provider_user_id, access_token, refresh_token, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (provider, provider_user_id) DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			expires_at = EXCLUDED.expires_at,
			updated_at = EXCLUDED.updated_at
	`, userID, provider, userInfo.ID, token.AccessToken, token.RefreshToken, token.Expiry, time.Now())

	if err != nil {
		return "", err
	}

	return userID, nil
}

type UserInfoResult struct {
	ID        string
	Email     string
	FullName  sql.NullString
	Phone     sql.NullString
	AvatarURL sql.NullString
}

func (db *DB) GetUserInfo(userID string) (*UserInfoResult, error) {
	var result UserInfoResult

	err := db.QueryRow(`
		SELECT u.id, u.email, ui.full_name, ui.phone, ui.avatar_url
		FROM users u
		LEFT JOIN user_infos ui ON u.id = ui.user_id
		WHERE u.id = $1
	`, userID).Scan(&result.ID, &result.Email, &result.FullName, &result.Phone, &result.AvatarURL)

	if err != nil {
		return nil, err
	}

	return &result, nil
}
