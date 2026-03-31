package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"portainerctl/internal/config"
	"portainerctl/internal/menu"
	"portainerctl/internal/model"
	"portainerctl/internal/portainer"
	"portainerctl/internal/render"
)

type options struct {
	url      string
	username string
	apiKey   string
	insecure bool
}

func NewRootCommand() *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "portainerctl",
		Short: "Interactive CLI for Portainer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), *opts)
		},
	}

	cmd.Flags().StringVar(&opts.url, "url", "", "Portainer base URL")
	cmd.Flags().StringVar(&opts.username, "username", "", "Portainer username")
	cmd.Flags().StringVar(&opts.apiKey, "api-key", "", "Portainer API key")
	cmd.Flags().BoolVar(&opts.insecure, "insecure", false, "Skip TLS certificate verification")
	cmd.AddCommand(newConfigCommand())
	cmd.AddCommand(newListEnvironmentsCommand(opts))
	cmd.AddCommand(newListContainersCommand(opts))
	cmd.AddCommand(newListStacksCommand(opts))
	cmd.AddCommand(newListImagesCommand(opts))

	return cmd
}

func run(ctx context.Context, opts options) error {
	render.Heading("Portainer Login")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	opts = mergeOptions(cfg, opts)
	opts, err = promptMissingOptions(opts)
	if err != nil {
		return err
	}

	client, cfg, opts, err := authenticate(ctx, cfg, opts, true)
	if err != nil {
		return err
	}

	app := menu.New(client)
	app.SetEnvironmentPickHandler(func(env model.Environment) error {
		cfg.DefaultEnvironmentID = env.ID
		cfg.DefaultEnvironment = env.Name
		return saveLoadedConfig(cfg)
	})
	return app.Run(ctx)
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
	opts.apiKey = strings.TrimSpace(opts.apiKey)
	return opts, nil
}

func mergeOptions(cfg config.Config, opts options) options {
	// CLI flags override saved config; API key also falls back to the env var for
	// non-interactive use in shells and CI.
	if strings.TrimSpace(opts.url) == "" {
		opts.url = cfg.URL
	}
	if strings.TrimSpace(opts.username) == "" {
		opts.username = cfg.Username
	}
	if strings.TrimSpace(opts.apiKey) == "" {
		opts.apiKey = firstNonEmpty(os.Getenv("PORTAINER_API_KEY"), cfg.APIKey)
	}
	return opts
}

func saveConfig(opts options) error {
	return saveLoadedConfig(config.Config{
		URL:      opts.url,
		Username: opts.username,
		APIKey:   opts.apiKey,
	})
}

func saveLoadedConfig(cfg config.Config) error {
	if err := config.Save(cfg); err != nil {
		return err
	}

	path, err := config.Path()
	if err != nil {
		return err
	}
	render.Successf("Saved config: %s", path)
	return nil
}

func maybeSaveAPIKey(opts options) (options, error) {
	if strings.TrimSpace(opts.apiKey) != "" {
		return opts, nil
	}

	var save bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Save an API key for future logins?",
		Default: false,
	}, &save); err != nil {
		return opts, err
	}
	if !save {
		return opts, nil
	}

	var apiKey string
	if err := survey.AskOne(&survey.Password{
		Message: "Portainer API key:",
	}, &apiKey); err != nil {
		return opts, err
	}

	opts.apiKey = strings.TrimSpace(apiKey)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View or update saved configuration",
	}

	cmd.AddCommand(newConfigViewCommand())
	cmd.AddCommand(newConfigClearCommand())
	cmd.AddCommand(newConfigSetCommand())
	return cmd
}

func newConfigViewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Show saved configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			path, err := config.Path()
			if err != nil {
				return err
			}

			render.Heading("Saved Config")
			fmt.Printf("Path: %s\n", path)
			fmt.Printf("URL: %s\n", emptyDisplay(cfg.URL))
			fmt.Printf("Username: %s\n", emptyDisplay(cfg.Username))
			fmt.Printf("API Key: %s\n", yesNoDisplay(strings.TrimSpace(cfg.APIKey) != ""))
			fmt.Printf("Default Environment ID: %s\n", emptyIntDisplay(cfg.DefaultEnvironmentID))
			fmt.Printf("Default Environment: %s\n", emptyDisplay(cfg.DefaultEnvironment))
			return nil
		},
	}
}

func newConfigClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove saved configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := config.Clear(); err != nil {
				return err
			}
			path, err := config.Path()
			if err != nil {
				return err
			}
			render.Successf("Cleared config: %s", path)
			return nil
		},
	}
}

func newConfigSetCommand() *cobra.Command {
	var (
		urlFlag      string
		usernameFlag string
		apiKeyFlag   string
		clearAPIKey  bool
		defaultEnvID int
		defaultEnv   string
		clearDefault bool
	)

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Update saved configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if strings.TrimSpace(urlFlag) != "" {
				normalizedURL, err := normalizePortainerURL(urlFlag)
				if err != nil {
					return err
				}
				cfg.URL = normalizedURL
			}
			if strings.TrimSpace(usernameFlag) != "" {
				cfg.Username = strings.TrimSpace(usernameFlag)
			}
			if clearAPIKey {
				cfg.APIKey = ""
			}
			if strings.TrimSpace(apiKeyFlag) != "" {
				cfg.APIKey = strings.TrimSpace(apiKeyFlag)
			}
			if clearDefault {
				cfg.DefaultEnvironmentID = 0
				cfg.DefaultEnvironment = ""
			}
			if defaultEnvID > 0 {
				cfg.DefaultEnvironmentID = defaultEnvID
			}
			if strings.TrimSpace(defaultEnv) != "" {
				cfg.DefaultEnvironment = strings.TrimSpace(defaultEnv)
			}

			if err := saveLoadedConfig(cfg); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&urlFlag, "url", "", "Save Portainer base URL")
	cmd.Flags().StringVar(&usernameFlag, "username", "", "Save Portainer username")
	cmd.Flags().StringVar(&apiKeyFlag, "api-key", "", "Save Portainer API key")
	cmd.Flags().BoolVar(&clearAPIKey, "clear-api-key", false, "Remove saved API key")
	cmd.Flags().IntVar(&defaultEnvID, "default-env-id", 0, "Save default environment ID")
	cmd.Flags().StringVar(&defaultEnv, "default-env", "", "Save default environment name")
	cmd.Flags().BoolVar(&clearDefault, "clear-default-env", false, "Remove saved default environment")
	return cmd
}

func emptyDisplay(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func yesNoDisplay(v bool) string {
	if v {
		return "set"
	}
	return "not set"
}

func emptyIntDisplay(v int) string {
	if v == 0 {
		return "-"
	}
	return strconv.Itoa(v)
}

func authenticate(ctx context.Context, cfg config.Config, opts options, allowPrompt bool) (*portainer.Client, config.Config, options, error) {
	var err error
	opts = mergeOptions(cfg, opts)
	if allowPrompt {
		opts, err = promptMissingOptions(opts)
		if err != nil {
			return nil, cfg, opts, err
		}
	} else {
		if strings.TrimSpace(opts.url) == "" {
			return nil, cfg, opts, fmt.Errorf("missing portainer url; set it with --url or `portainerctl config set --url ...`")
		}
		if strings.TrimSpace(opts.username) == "" && strings.TrimSpace(opts.apiKey) == "" {
			return nil, cfg, opts, fmt.Errorf("missing username or api key; set one with flags or `portainerctl config set`")
		}
		opts.url, err = normalizePortainerURL(opts.url)
		if err != nil {
			return nil, cfg, opts, err
		}
		opts.username = strings.TrimSpace(opts.username)
		opts.apiKey = strings.TrimSpace(opts.apiKey)
	}

	client, err := portainer.NewClient(opts.url, opts.insecure)
	if err != nil {
		return nil, cfg, opts, err
	}

	// Prefer API-key auth when available so one-liner commands can run without a
	// password prompt. Falling back to password keeps the interactive flow usable
	// if the saved key has expired or been revoked.
	if strings.TrimSpace(opts.apiKey) != "" {
		client.SetAPIKey(opts.apiKey)
		if _, err := client.ListEnvironments(ctx); err == nil {
			render.Successf("Authentication successful with API key.")
			cfg.URL = opts.url
			cfg.Username = opts.username
			cfg.APIKey = opts.apiKey
			if err := saveLoadedConfig(cfg); err != nil {
				render.Warningf("Could not save config: %v", err)
			}
			return client, cfg, opts, nil
		}
		if !allowPrompt {
			return nil, cfg, opts, fmt.Errorf("api key authentication failed")
		}
		render.Warningf("Saved API key authentication failed. Falling back to password login.")
	}

	if !allowPrompt {
		password, err := promptPassword()
		if err != nil {
			return nil, cfg, opts, err
		}
		if err := client.Login(ctx, opts.username, password); err != nil {
			return nil, cfg, opts, err
		}
		render.Successf("Authentication successful.")
		cfg.URL = opts.url
		cfg.Username = opts.username
		if err := saveLoadedConfig(cfg); err != nil {
			render.Warningf("Could not save config: %v", err)
		}
		return client, cfg, opts, nil
	}

	for attempt := 1; attempt <= 3; attempt++ {
		password, err := promptPassword()
		if err != nil {
			return nil, cfg, opts, err
		}

		if err := client.Login(ctx, opts.username, password); err != nil {
			if attempt == 3 {
				return nil, cfg, opts, fmt.Errorf("authentication failed after 3 attempts")
			}
			render.Errorf("Authentication failed (%d/3).", attempt)
			continue
		}

		render.Successf("Authentication successful.")
		opts, err = maybeSaveAPIKey(opts)
		if err != nil {
			return nil, cfg, opts, err
		}
		cfg.URL = opts.url
		cfg.Username = opts.username
		cfg.APIKey = opts.apiKey
		if err := saveLoadedConfig(cfg); err != nil {
			render.Warningf("Could not save config: %v", err)
		}
		return client, cfg, opts, nil
	}

	return nil, cfg, opts, fmt.Errorf("authentication failed after 3 attempts")
}

func newListEnvironmentsCommand(rootOpts *options) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list-environments",
		Short: "List Portainer environments",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			client, _, _, err := authenticate(cmd.Context(), cfg, *rootOpts, false)
			if err != nil {
				return err
			}
			envs, err := client.ListEnvironments(cmd.Context())
			if err != nil {
				return err
			}
			return outputEnvironments(envs, format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table or json")
	return cmd
}

func newListContainersCommand(rootOpts *options) *cobra.Command {
	return newEnvironmentListCommand(rootOpts, "list-containers", "List containers from the default environment", func(ctx context.Context, client *portainer.Client, env model.Environment) (any, error) {
		return client.ListContainers(ctx, env.ID)
	}, outputContainers)
}

func newListStacksCommand(rootOpts *options) *cobra.Command {
	return newEnvironmentListCommand(rootOpts, "list-stacks", "List stacks from the default environment", func(ctx context.Context, client *portainer.Client, env model.Environment) (any, error) {
		return client.ListStacks(ctx, env.ID)
	}, outputStacks)
}

func newListImagesCommand(rootOpts *options) *cobra.Command {
	return newEnvironmentListCommand(rootOpts, "list-images", "List images from the default environment", func(ctx context.Context, client *portainer.Client, env model.Environment) (any, error) {
		return client.ListImages(ctx, env.ID)
	}, outputImages)
}

func newEnvironmentListCommand(rootOpts *options, use, short string, fetch func(context.Context, *portainer.Client, model.Environment) (any, error), output func(any, string) error) *cobra.Command {
	var (
		format  string
		envID   int
		envName string
	)
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			client, cfg, _, err := authenticate(cmd.Context(), cfg, *rootOpts, false)
			if err != nil {
				return err
			}
			env, err := resolveEnvironment(cmd.Context(), client, cfg, envID, envName)
			if err != nil {
				return err
			}
			data, err := fetch(cmd.Context(), client, env)
			if err != nil {
				return err
			}
			return output(data, format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table or json")
	cmd.Flags().IntVar(&envID, "env-id", 0, "Environment ID")
	cmd.Flags().StringVar(&envName, "env", "", "Environment name")
	return cmd
}

func resolveEnvironment(ctx context.Context, client *portainer.Client, cfg config.Config, envID int, envName string) (model.Environment, error) {
	envs, err := client.ListEnvironments(ctx)
	if err != nil {
		return model.Environment{}, err
	}

	// Resolution order is explicit flags first, then saved defaults from the
	// interactive picker or config command.
	if envID > 0 {
		for _, env := range envs {
			if env.ID == envID {
				return env, nil
			}
		}
		return model.Environment{}, fmt.Errorf("environment id %d not found", envID)
	}

	if strings.TrimSpace(envName) != "" {
		for _, env := range envs {
			if strings.EqualFold(env.Name, strings.TrimSpace(envName)) {
				return env, nil
			}
		}
		return model.Environment{}, fmt.Errorf("environment %q not found", envName)
	}

	if cfg.DefaultEnvironmentID > 0 {
		for _, env := range envs {
			if env.ID == cfg.DefaultEnvironmentID {
				return env, nil
			}
		}
	}

	if strings.TrimSpace(cfg.DefaultEnvironment) != "" {
		for _, env := range envs {
			if strings.EqualFold(env.Name, strings.TrimSpace(cfg.DefaultEnvironment)) {
				return env, nil
			}
		}
	}

	return model.Environment{}, fmt.Errorf("no default environment configured; use --env-id, --env, or select one in the interactive app")
}

func outputEnvironments(envs []model.Environment, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return printJSON(envs)
	case "table":
		render.Heading("Environments")
		render.RenderEnvironments(envs)
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func outputContainers(data any, format string) error {
	containers, ok := data.([]model.Container)
	if !ok {
		return fmt.Errorf("unexpected containers payload")
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return printJSON(containers)
	case "table":
		render.Heading("Containers")
		render.RenderContainers(containers)
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func outputStacks(data any, format string) error {
	stacks, ok := data.([]model.Stack)
	if !ok {
		return fmt.Errorf("unexpected stacks payload")
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return printJSON(stacks)
	case "table":
		render.Heading("Stacks")
		render.RenderStacks(stacks)
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func outputImages(data any, format string) error {
	images, ok := data.([]model.Image)
	if !ok {
		return fmt.Errorf("unexpected images payload")
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return printJSON(images)
	case "table":
		render.Heading("Images")
		render.RenderImages(images)
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
