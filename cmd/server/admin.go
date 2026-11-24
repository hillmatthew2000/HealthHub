package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/hillmatthew2000/HealthHub/internal/auth"
	"github.com/hillmatthew2000/HealthHub/internal/handlers"
	"github.com/hillmatthew2000/HealthHub/internal/models"
	"github.com/hillmatthew2000/HealthHub/internal/store"
	"github.com/hillmatthew2000/HealthHub/pkg/logger"
)

// adminPasswordEnv names the variable create-admin reads the password from.
// Passwords are never taken as flags, which would expose them in the
// process list and shell history.
const adminPasswordEnv = "ADMIN_PASSWORD" // #nosec G101 -- the name of a variable, not a credential

// createAdmin creates an administrator account, so a new installation can
// be bootstrapped without enabling self-registration.
func createAdmin(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
	fs.SetOutput(stdout)
	var req models.CreateUserRequest
	fs.StringVar(&req.Email, "email", "", "email address of the account (required)")
	fs.StringVar(&req.FirstName, "first-name", "", "first name (required)")
	fs.StringVar(&req.LastName, "last-name", "", "last name (required)")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: server create-admin -email ADDRESS -first-name NAME -last-name NAME\n\n"+
			"The password is read from %s or, if that is unset, from the first line\nof standard input.\n\n", adminPasswordEnv)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("create-admin: unexpected argument %q", fs.Arg(0))
	}

	password, err := readPassword(stdin)
	if err != nil {
		return err
	}
	req.Email = handlers.NormalizeEmail(req.Email)
	req.FirstName, req.LastName = strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName)
	req.Password = password
	req.Roles = []string{string(auth.RoleAdmin)}
	if err := validateAdmin(&req); err != nil {
		return err
	}

	cfg, err := setup()
	if err != nil {
		return err
	}
	defer logger.Sync()
	passwords, err := auth.NewPasswordHasher(cfg.BcryptCost)
	if err != nil {
		return err
	}
	hash, err := passwords.Hash(req.Password)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := openDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if cfg.AutoMigrate {
		if err := applyMigrations(ctx, db); err != nil {
			return err
		}
	}

	user := &models.User{
		Email:        req.Email,
		PasswordHash: hash,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Roles:        req.Roles,
		Active:       true,
	}
	if err := store.NewUsers(db).Create(ctx, user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return fmt.Errorf("create-admin: an account for %s already exists", req.Email)
		}
		return err
	}
	logger.LogSecurityEvent("user_created", user.ID, map[string]any{"roles": user.Roles, "by": "create-admin"})
	_, err = fmt.Fprintf(stdout, "created administrator %s (%s)\n", user.Email, user.ID)
	return err
}

func readPassword(stdin io.Reader) (string, error) {
	if p := os.Getenv(adminPasswordEnv); p != "" {
		return p, nil
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("create-admin: read password: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", fmt.Errorf("create-admin: no password given; set %s or pipe it on standard input", adminPasswordEnv)
	}
	return line, nil
}

func validateAdmin(req *models.CreateUserRequest) error {
	if err := auth.CheckPolicy(req.Password); err != nil {
		return fmt.Errorf("create-admin: %w", err)
	}
	err := validator.New(validator.WithRequiredStructEnabled()).Struct(req)
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err
	}
	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		msgs = append(msgs, flagName(fe.Field())+" is invalid ("+fe.Tag()+")")
	}
	return errors.New("create-admin: " + strings.Join(msgs, "; "))
}

// flagName maps a request field to the flag that sets it.
func flagName(field string) string {
	switch field {
	case "Email":
		return "-email"
	case "FirstName":
		return "-first-name"
	case "LastName":
		return "-last-name"
	}
	return strings.ToLower(field)
}
