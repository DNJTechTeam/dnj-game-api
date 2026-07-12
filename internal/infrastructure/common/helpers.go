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

// GenerateVerificationCode produces a random 6-digit numeric code (zero
// padded) used by the passwordless onboarding flow.
func GenerateVerificationCode() string {
	const digits = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		code[i] = digits[idx.Int64()]
	}
	return string(code)
}
