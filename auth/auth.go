package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

type AuthModule struct {
	db        *pgxpool.Pool
	redis     *redis.Client
	JWTSecret string
}

func NewAuthModule(db *pgxpool.Pool, redis *redis.Client, JWTSecret string) *AuthModule {
	return &AuthModule{
		db:        db,
		redis:     redis,
		JWTSecret: JWTSecret,
	}
}

func generateSecureToken(length int) (string, error) {
	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(randomBytes), nil
}

func (a *AuthModule) createUser(ctx context.Context, username, password, email string) (int, error) {
	var exists bool
	err := a.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", username).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errors.New("username already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	var userID int
	err = a.db.QueryRow(ctx,
		"INSERT INTO users (username, password, email) VALUES ($1, $2, $3) RETURNING id",
		username, string(hashedPassword), email,
	).Scan(&userID)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

func (a *AuthModule) generateJWT(userID int) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.JWTSecret))
}

func (a *AuthModule) authenticateUser(ctx context.Context, username string, password string) (int, error) {
	var userID int
	var passwordHash string
	err := a.db.QueryRow(ctx, "SELECT id, password FROM users WHERE username = $1", username).Scan(&userID, &passwordHash)
	if err != nil {
		return 0, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return 0, errors.New("invalid credentials")
	}

	return userID, nil
}

func (a *AuthModule) RegisterWithJWT(ctx context.Context, username string, password string, email string) (string, error) {
	userID, err := a.createUser(ctx, username, password, email)
	if err != nil {
		return "", err
	}

	token, err := a.generateJWT(userID)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (a *AuthModule) RegisterWithSession(ctx context.Context, username, password string, email string) (int, string, error) {
	userID, err := a.createUser(ctx, username, password, email)
	if err != nil {
		return 0, "", err
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return 0, "", err
	}

	key := "session:" + token
	err = a.redis.Set(ctx, key, userID, 24*time.Hour).Err()
	if err != nil {
		return 0, "", err
	}

	return userID, token, nil
}

func (a *AuthModule) LoginWithJWT(ctx context.Context, username, password string) (string, error) {
	userID, err := a.authenticateUser(ctx, username, password)
	if err != nil {
		return "", err
	}

	token, err := a.generateJWT(userID)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (a *AuthModule) LoginWithSession(ctx context.Context, username, password string) (int, string, error) {
	userID, err := a.authenticateUser(ctx, username, password)
	if err != nil {
		return 0, "", err
	}

	token, err := generateSecureToken(32)
	if err != nil {
		return 0, "", err
	}

	key := "session:" + token
	err = a.redis.Set(ctx, key, userID, 24*time.Hour).Err()
	if err != nil {
		return 0, "", err
	}

	return userID, token, nil
}

func (a *AuthModule) ValidateTokenJWT(ctx context.Context, token string) (string, error) {
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(a.JWTSecret), nil
	})
	if err != nil {
		return "", err
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok && parsedToken.Valid {
		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			return "", errors.New("invalid user_id in token")
		}
		userID := int(userIDFloat)
		return fmt.Sprintf("%d", userID), nil
	}

	return "", errors.New("invalid token")
}

func (a *AuthModule) ValidateTokenSession(ctx context.Context, token string) (string, error) {
	key := "session:" + token
	userID, err := a.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", errors.New("invalid token")
	} else if err != nil {
		return "", err
	}

	// Check the expiration time of the token
	ttl, err := a.redis.TTL(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return "", err
	}

	// Update expiration only after some time
	if ttl < 20*time.Hour {
		err = a.redis.Expire(ctx, key, 24*time.Hour).Err()
		if err != nil {
			return "", err
		}
	}
	return userID, nil
}

func (a *AuthModule) LogoutJWT(ctx context.Context, token string) error {
	// For JWT, logout is handled client-side by discarding the token
	return nil
}

func (a *AuthModule) LogoutSession(ctx context.Context, token string) error {
	key := "session:" + token
	return a.redis.Del(ctx, key).Err()
}

// ChangePassword changes the user's password after verifying the old password
func (a *AuthModule) ChangePassword(ctx context.Context, userID int, oldPassword, newPassword string) error {
	var passwordHash string
	err := a.db.QueryRow(ctx, "SELECT password FROM users WHERE id = $1", userID).Scan(&passwordHash)
	if err != nil {
		return errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(oldPassword)); err != nil {
		return errors.New("invalid old password")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = a.db.Exec(ctx, "UPDATE users SET password = $1 WHERE id = $2", string(hashedPassword), userID)
	return err
}

// ChangeEmail changes the user's email after verifying the password
func (a *AuthModule) ChangeEmail(ctx context.Context, userID int, password, newEmail string) error {
	var passwordHash string
	err := a.db.QueryRow(ctx, "SELECT password FROM users WHERE id = $1", userID).Scan(&passwordHash)
	if err != nil {
		return errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return errors.New("invalid password")
	}

	_, err = a.db.Exec(ctx, "UPDATE users SET email = $1 WHERE id = $2", newEmail, userID)
	return err
}

//
// TOKEN SIGNING FOR HASHED TOKENS IN REDIS
//
// func signToken(token, secretKey string) string {
// 	h := hmac.New(sha256.New, []byte(secretKey))
// 	h.Write([]byte(token))
// 	signature := h.Sum(nil)

// 	return base64.URLEncoding.EncodeToString(signature)
// }
