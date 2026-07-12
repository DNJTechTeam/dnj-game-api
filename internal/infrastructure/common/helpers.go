package common

import (
	"context"
	"crypto/rand"
	"log"
	"math/big"
	"net/mail"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type Environment string

const (
	EnvironmentLocalhost   Environment = "localhost"
	EnvironmentDevelopment Environment = "development"
	EnvironmentProduction  Environment = "production"
	EnvironmentTest        Environment = "test"
)

// UserIDContextKey is the key under which AuthenticationMiddleware stores the
// authenticated user id (as a string) in the request context.
const UserIDContextKey = "userId"

func GetEnv(env string) string {
	value, isSet := os.LookupEnv(env)

	if !isSet {
		log.Panicf("environment variable not set: %s", env)
	}

	return value
}

func EnvironmentIs(env Environment) bool {
	return GetEnv("SERVER_ENVIRONMENT") == string(env)
}

func WaitOsInterruption() {
	var waitGroup sync.WaitGroup

	osInterrupt := make(chan os.Signal, 1)
	signal.Notify(osInterrupt, os.Interrupt)

	syscallSigterm := make(chan os.Signal, 1)
	signal.Notify(syscallSigterm, syscall.SIGTERM)

	waitGroup.Add(1)

	go func() {
		<-osInterrupt
		defer waitGroup.Done()
	}()

	go func() {
		<-syscallSigterm
		defer waitGroup.Done()
	}()

	waitGroup.Wait()
}

func ValidateEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func SanitizePhone(phone string) string {
	re := regexp.MustCompile(`\D`)
	return re.ReplaceAllString(phone, "")
}

func ValidatePhone(phone string, isMobile bool) bool {
	sanitizedPhone := SanitizePhone(phone)
	length := len(sanitizedPhone)

	if isMobile {
		return length >= 11 && length <= 13
	}

	return length >= 10 && length <= 12
}

func ValidatePasswordComplexity(password string) bool {
	hasRequiredSize := len(password) >= 8
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)

	return hasRequiredSize && hasUpper && hasLower && hasNumber && hasSpecial
}

func CleanString(s string) string {
	s = strings.Replace(s, ".", "", -1)
	s = strings.Replace(s, "-", "", -1)
	s = strings.Replace(s, "/", "", -1)
	return s
}

// ExtractUserIdFromContext returns the authenticated user id placed in the
// context by AuthenticationMiddleware, or 0 when there is none.
func ExtractUserIdFromContext(ctx context.Context) uint64 {
	userIDFromContext := ctx.Value(UserIDContextKey)
	if userIDFromContext == nil {
		return 0
	}

	userIDString, ok := userIDFromContext.(string)
	if !ok {
		return 0
	}

	userId, err := strconv.ParseUint(userIDString, 10, 64)
	if err != nil {
		return 0
	}
	return userId
}

// GenerateRandomPassword produces a 12-char password containing at least one
// lowercase, uppercase, digit and special character. Used by the password
// recovery flow to issue a temporary password.
func GenerateRandomPassword() string {
	const (
		lowercaseChars = "abcdefghijklmnopqrstuvwxyz"
		uppercaseChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		numberChars    = "0123456789"
		specialChars   = "!@#$%^&*()_+-=[]{};:,.<>/\\"
		maxLength      = 12
	)

	allChars := lowercaseChars + uppercaseChars + numberChars + specialChars
	password := make([]byte, maxLength)

	charSets := []string{lowercaseChars, uppercaseChars, numberChars, specialChars}
	for i, charSet := range charSets {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charSet))))
		password[i] = charSet[idx.Int64()]
	}

	for i := len(charSets); i < maxLength; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(allChars))))
		password[i] = allChars[idx.Int64()]
	}

	for i := len(password) - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		password[i], password[j.Int64()] = password[j.Int64()], password[i]
	}

	return string(password)
}
