package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"portainerctl/internal/menu"
	"portainerctl/internal/portainer"
	"portainerctl/internal/render"
)

type options struct {
	url      string
	username string
	insecure bool
}

func NewRootCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "portainerctl --url <portainer-url> --username <username>",
		Short: "Interactive CLI for Portainer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), *opts)
		},
	}

	cmd.Flags().StringVar(&opts.url, "url", "", "Portainer base URL")
	cmd.Flags().StringVar(&opts.username, "username", "", "Portainer username")
	cmd.Flags().BoolVar(&opts.insecure, "insecure", false, "Skip TLS certificate verification")

	return cmd
}

func run(ctx context.Context, opts options) error {
	render.Heading("Portainer Login")

	opts, err := promptMissingOptions(opts)
	if err != nil {
		return err
	}

	client, err := portainer.NewClient(opts.url, opts.insecure)
	if err != nil {
		return err
	}

	for attempt := 1; attempt <= 3; attempt++ {
		password, err := promptPassword()
		if err != nil {
			return err
		}

		if err := client.Login(ctx, opts.username, password); err != nil {
			if attempt == 3 {
				return fmt.Errorf("authentication failed after 3 attempts")
			}
			render.Errorf("Authentication failed (%d/3).", attempt)
			continue
		}

		render.Successf("Authentication successful.")
		app := menu.New(client)
		return app.Run(ctx)
	}

	return fmt.Errorf("authentication failed after 3 attempts")
}

func promptPassword() (string, error) {
	var password string
	err := survey.AskOne(&survey.Password{Message: "Password:"}, &password, survey.WithValidator(survey.Required))
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	value := strings.TrimSpace(password)
	if value == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	return value, nil
}

func promptMissingOptions(opts options) (options, error) {
	if strings.TrimSpace(opts.url) == "" {
		if err := survey.AskOne(&survey.Input{Message: "Portainer URL:"}, &opts.url, survey.WithValidator(survey.Required)); err != nil {
			return opts, err
		}
	}

	if strings.TrimSpace(opts.username) == "" {
		if err := survey.AskOne(&survey.Input{Message: "Username:"}, &opts.username, survey.WithValidator(survey.Required)); err != nil {
			return opts, err
		}
	}

	normalizedURL, err := normalizePortainerURL(opts.url)
	if err != nil {
		return opts, err
	}
	opts.url = normalizedURL
	opts.username = strings.TrimSpace(opts.username)
	return opts, nil
}

func normalizePortainerURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("portainer url cannot be empty")
	}

	if !strings.Contains(value, "://") {
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid portainer url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid portainer url: %q", raw)
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
