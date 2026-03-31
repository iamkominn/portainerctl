package menu

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"

	"portainerctl/internal/model"
	"portainerctl/internal/portainer"
	"portainerctl/internal/render"
)

type App struct {
	client            *portainer.Client
	onEnvironmentPick func(model.Environment) error
}

const actionDivider = "──────── Actions ────────"

func New(client *portainer.Client) *App {
	return &App{client: client}
}

func (a *App) SetEnvironmentPickHandler(fn func(model.Environment) error) {
	a.onEnvironmentPick = fn
}

func (a *App) Run(ctx context.Context) error {
	envs, err := a.client.ListEnvironments(ctx)
	if err != nil {
		return err
	}
	if len(envs) == 0 {
		return fmt.Errorf("no environments available")
	}

	selectedEnv, err := a.selectEnvironment(envs)
	if err != nil {
		return err
	}
	if selectedEnv.ID == 0 && selectedEnv.Name == "" {
		return nil
	}
	if a.onEnvironmentPick != nil {
		// The CLI remembers the last selected environment so non-interactive list
		// commands can target it without an explicit flag.
		if err := a.onEnvironmentPick(selectedEnv); err != nil {
			render.Warningf("Could not save default environment: %v", err)
		}
	}

	for {
		render.Heading("Environment: " + selectedEnv.Name)
		choice, err := selectOption("Choose a resource", []string{"Containers", "Stacks", "Images", "Switch Environment", "Exit"})
		if err != nil {
			return err
		}

		switch choice {
		case "Containers":
			if err := a.runContainers(ctx, selectedEnv); err != nil {
				render.Errorf("%v", err)
			}
		case "Stacks":
			if err := a.runStacks(ctx, selectedEnv); err != nil {
				render.Errorf("%v", err)
			}
		case "Images":
			if err := a.runImages(ctx, selectedEnv); err != nil {
				render.Errorf("%v", err)
			}
		case "Switch Environment":
			selectedEnv, err = a.selectEnvironment(envs)
			if err != nil {
				return err
			}
			if selectedEnv.ID == 0 && selectedEnv.Name == "" {
				return nil
			}
			if a.onEnvironmentPick != nil {
				if err := a.onEnvironmentPick(selectedEnv); err != nil {
					render.Warningf("Could not save default environment: %v", err)
				}
			}
		case "Exit":
			return nil
		}
	}
}

func (a *App) selectEnvironment(envs []model.Environment) (model.Environment, error) {
	render.Heading("Environments")
	render.RenderEnvironments(envs)

	options := make([]string, 0, len(envs))
	lookup := make(map[string]model.Environment, len(envs))
	for _, env := range envs {
		label := fmt.Sprintf("%s [%s]", env.Name, renderEnvironmentType(env.Type))
		options = append(options, label)
		lookup[label] = env
	}
	options = append(options, actionDivider, actionLabel("Exit"))

	choice, err := selectOption("Select an environment", options)
	if err != nil {
		return model.Environment{}, err
	}
	if isAction(choice, "Exit") {
		return model.Environment{}, nil
	}
	return lookup[choice], nil
}

func (a *App) runContainers(ctx context.Context, env model.Environment) error {
	for {
		containers, err := a.client.ListContainers(ctx, env.ID)
		if err != nil {
			return err
		}

		render.Heading("Containers")
		render.RenderContainers(containers)
		if len(containers) == 0 {
			render.Warningf("No containers found.")
			choice, err := selectOption("Container menu", []string{actionLabel("Refresh"), actionLabel("Back"), actionLabel("Exit")})
			if err != nil {
				return err
			}
			if isAction(choice, "Back") {
				return nil
			}
			if isAction(choice, "Exit") {
				return nil
			}
			continue
		}

		options := make([]string, 0, len(containers)+2)
		lookup := make(map[string]model.Container, len(containers))
		for _, c := range containers {
			label := fmt.Sprintf("%s [%s]", renderContainerName(c), render.ContainerState(c.State))
			options = append(options, label)
			lookup[label] = c
		}
		options = append(options, actionDivider, actionLabel("Refresh"), actionLabel("Back"), actionLabel("Exit"))

		choice, err := selectOption("Select a container", options)
		if err != nil {
			return err
		}
		if isAction(choice, "Back") {
			return nil
		}
		if isAction(choice, "Exit") {
			return nil
		}
		if isAction(choice, "Refresh") {
			continue
		}

		c := lookup[choice]
		action, err := selectOption("Container action", []string{"Start", "Stop", "Restart", "Remove", actionLabel("Back"), actionLabel("Exit")})
		if err != nil {
			return err
		}
		if isAction(action, "Back") {
			continue
		}
		if isAction(action, "Exit") {
			return nil
		}

		if action == "Remove" {
			ok, err := confirm("Remove container " + renderContainerName(c) + "?")
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
		}

		switch action {
		case "Start":
			err = a.client.StartContainer(ctx, env.ID, c.ID)
		case "Stop":
			err = a.client.StopContainer(ctx, env.ID, c.ID)
		case "Restart":
			err = a.client.RestartContainer(ctx, env.ID, c.ID)
		case "Remove":
			err = a.client.RemoveContainer(ctx, env.ID, c.ID)
		}
		if err != nil {
			render.Errorf("%v", err)
			continue
		}
		render.Successf("%s %s completed.", action, renderContainerName(c))
	}
}

func (a *App) runStacks(ctx context.Context, env model.Environment) error {
	for {
		stacks, err := a.client.ListStacks(ctx, env.ID)
		if err != nil {
			return err
		}

		render.Heading("Stacks")
		render.RenderStacks(stacks)
		if len(stacks) == 0 {
			render.Warningf("No stacks found.")
			choice, err := selectOption("Stack menu", []string{actionLabel("Refresh"), actionLabel("Back"), actionLabel("Exit")})
			if err != nil {
				return err
			}
			if isAction(choice, "Back") {
				return nil
			}
			if isAction(choice, "Exit") {
				return nil
			}
			continue
		}

		options := make([]string, 0, len(stacks)+2)
		lookup := make(map[string]model.Stack, len(stacks))
		for _, stack := range stacks {
			label := fmt.Sprintf("%s [%s]", stack.Name, render.StackStatus(stack.Status))
			options = append(options, label)
			lookup[label] = stack
		}
		options = append(options, actionDivider, actionLabel("Refresh"), actionLabel("Back"), actionLabel("Exit"))

		choice, err := selectOption("Select a stack", options)
		if err != nil {
			return err
		}
		if isAction(choice, "Back") {
			return nil
		}
		if isAction(choice, "Exit") {
			return nil
		}
		if isAction(choice, "Refresh") {
			continue
		}

		stack := lookup[choice]
		if stack.Limited {
			// Limited stacks are inferred from Docker Compose labels and do not map
			// cleanly to the Portainer-managed stack lifecycle endpoints.
			render.Warningf("Limited stacks are detected from Docker Compose labels and are not fully manageable via the current Portainer stack API flow.")
			action, err := selectOption("Stack action", []string{actionLabel("Back"), actionLabel("Exit")})
			if err != nil {
				return err
			}
			if isAction(action, "Exit") {
				return nil
			}
			continue
		}

		action, err := selectOption("Stack action", []string{"Start", "Stop", "Remove", actionLabel("Back"), actionLabel("Exit")})
		if err != nil {
			return err
		}
		if isAction(action, "Back") {
			continue
		}
		if isAction(action, "Exit") {
			return nil
		}

		if action == "Remove" {
			ok, err := confirm("Remove stack " + stack.Name + "?")
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
		}

		switch action {
		case "Start":
			err = a.client.StartStack(ctx, stack.ID, env.ID)
		case "Stop":
			err = a.client.StopStack(ctx, stack.ID, env.ID)
		case "Remove":
			err = a.client.RemoveStack(ctx, stack.ID, env.ID)
		}
		if err != nil {
			render.Errorf("%v", err)
			continue
		}
		render.Successf("%s %s completed.", action, stack.Name)
	}
}

func (a *App) runImages(ctx context.Context, env model.Environment) error {
	for {
		images, err := a.client.ListImages(ctx, env.ID)
		if err != nil {
			return err
		}

		render.Heading("Images")
		render.RenderImages(images)
		if len(images) == 0 {
			render.Warningf("No images found.")
			choice, err := selectOption("Image menu", []string{actionLabel("Refresh"), actionLabel("Back"), actionLabel("Exit")})
			if err != nil {
				return err
			}
			if isAction(choice, "Back") {
				return nil
			}
			if isAction(choice, "Exit") {
				return nil
			}
			continue
		}

		options := make([]string, 0, len(images)+2)
		lookup := make(map[string]model.Image, len(images))
		for _, image := range images {
			label := renderImageName(image)
			options = append(options, label)
			lookup[label] = image
		}
		options = append(options, actionDivider, actionLabel("Refresh"), actionLabel("Back"), actionLabel("Exit"))

		choice, err := selectOption("Select an image", options)
		if err != nil {
			return err
		}
		if isAction(choice, "Back") {
			return nil
		}
		if isAction(choice, "Exit") {
			return nil
		}
		if isAction(choice, "Refresh") {
			continue
		}

		image := lookup[choice]
		action, err := selectOption("Image action", []string{"Remove", actionLabel("Back"), actionLabel("Exit")})
		if err != nil {
			return err
		}
		if isAction(action, "Back") {
			continue
		}
		if isAction(action, "Exit") {
			return nil
		}

		ok, err := confirm("Remove image " + renderImageName(image) + "?")
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		if err := a.client.RemoveImage(ctx, env.ID, image.ID); err != nil {
			render.Errorf("%v", err)
			continue
		}
		render.Successf("Removed image %s.", renderImageName(image))
	}
}

func selectOption(message string, options []string) (string, error) {
	var selection string
	prompt := &survey.Select{
		Message:  message,
		Options:  options,
		PageSize: 12,
	}
	if err := survey.AskOne(prompt, &selection); err != nil {
		return "", err
	}
	if selection == actionDivider {
		return selectOption(message, options)
	}
	return selection, nil
}

func confirm(message string) (bool, error) {
	var confirmed bool
	if err := survey.AskOne(&survey.Confirm{Message: message, Default: false}, &confirmed); err != nil {
		return false, err
	}
	return confirmed, nil
}

func renderEnvironmentType(v int) string {
	switch v {
	case 1:
		return "Docker"
	case 2:
		return "Agent"
	case 3:
		return "Azure"
	case 4:
		return "Edge"
	case 5:
		return "K8s"
	default:
		return fmt.Sprintf("%d", v)
	}
}

func renderContainerName(c model.Container) string {
	if len(c.Names) == 0 {
		return c.ID
	}
	return c.Names[0][1:]
}

func renderStackStatus(v int) string {
	switch v {
	case 1:
		return "active"
	case 2:
		return "inactive"
	default:
		return fmt.Sprintf("%d", v)
	}
}

func renderImageName(img model.Image) string {
	if len(img.RepoTags) > 0 {
		return img.RepoTags[0]
	}
	return img.ID
}

func actionLabel(label string) string {
	return "[" + label + "]"
}

func isAction(choice, label string) bool {
	return strings.EqualFold(choice, label) || strings.EqualFold(choice, actionLabel(label))
}
