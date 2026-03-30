package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"

	"portainerctl/internal/model"
)

var (
	headerColor = color.New(color.FgCyan, color.Bold)
	lineColor   = color.New(color.FgHiBlack)
	okColor     = color.New(color.FgGreen, color.Bold)
	warnColor   = color.New(color.FgYellow, color.Bold)
	errColor    = color.New(color.FgRed, color.Bold)
)

func Heading(title string) {
	fmt.Println()
	headerColor.Println(title)
	lineColor.Println(strings.Repeat("─", len(title)))
}

func Successf(format string, args ...any) {
	okColor.Printf(format+"\n", args...)
}

func Warningf(format string, args ...any) {
	warnColor.Printf(format+"\n", args...)
}

func Errorf(format string, args ...any) {
	errColor.Printf(format+"\n", args...)
}

func RenderEnvironments(envs []model.Environment) {
	tw := newTable()
	tw.AppendHeader(table.Row{"#", "ID", "Name", "Type", "URL", "TLS"})
	for i, env := range envs {
		tw.AppendRow(table.Row{i + 1, env.ID, env.Name, environmentType(env.Type), firstNonEmpty(env.PublicURL, env.URL), yesNo(env.TLS)})
	}
	fmt.Println(tw.Render())
}

func RenderContainers(containers []model.Container) {
	tw := newTable()
	tw.AppendHeader(table.Row{"#", "Name", "State", "Image", "Ports", "Created"})
	for i, container := range containers {
		tw.AppendRow(table.Row{
			i + 1,
			containerName(container),
			ContainerState(container.State),
			container.Image,
			renderPorts(container.Ports),
			formatUnix(container.Created),
		})
	}
	fmt.Println(tw.Render())
}

func RenderStacks(stacks []model.Stack) {
	tw := newTable()
	tw.AppendHeader(table.Row{"#", "Name", "Status", "Control", "Created Date", "Updated Date"})
	for i, stack := range stacks {
		tw.AppendRow(table.Row{
			i + 1,
			stack.Name,
			StackStatus(stack.Status),
			coloredStackControl(stack),
			formatUnix(stack.CreationDate),
			formatUnix(stack.UpdateDate),
		})
	}
	fmt.Println(tw.Render())
}

func RenderImages(images []model.Image) {
	tw := newTable()
	tw.AppendHeader(table.Row{"#", "ID", "Repository:Tag", "Size", "Created"})
	for i, image := range images {
		tw.AppendRow(table.Row{
			i + 1,
			shortID(image.ID),
			imageName(image),
			humanSize(image.Size),
			formatUnix(image.Created),
		})
	}
	fmt.Println(tw.Render())
}

func newTable() table.Writer {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleRounded)
	return tw
}

func environmentType(v int) string {
	switch v {
	case 1:
		return "Docker"
	case 2:
		return "Agent"
	case 3:
		return "Azure"
	case 4:
		return "Edge Agent"
	case 5:
		return "Kubernetes"
	default:
		return fmt.Sprintf("%d", v)
	}
}

func stackType(v int) string {
	switch v {
	case 1:
		return "Swarm"
	case 2:
		return "Compose"
	case 3:
		return "Kubernetes"
	default:
		return fmt.Sprintf("%d", v)
	}
}

func coloredStackControl(stack model.Stack) string {
	switch stackControl(stack) {
	case "Full":
		return okColor.Sprint("Full")
	case "Limited":
		return warnColor.Sprint("Limited")
	default:
		return stackControl(stack)
	}
}

func stackControl(stack model.Stack) string {
	if strings.TrimSpace(stack.Origin) != "" {
		return stack.Origin
	}
	if strings.TrimSpace(stack.CreatedBy) != "" {
		return "Full"
	}
	if stack.ResourceControl != nil {
		return "Full"
	}
	return "Limited"
}

func ContainerState(state string) string {
	switch strings.ToLower(state) {
	case "running":
		return okColor.Sprint(state)
	case "paused":
		return warnColor.Sprint(state)
	default:
		return errColor.Sprint(state)
	}
}

func StackStatus(v int) string {
	switch v {
	case 1:
		return okColor.Sprint("active")
	case 2:
		return warnColor.Sprint("inactive")
	default:
		return fmt.Sprintf("%d", v)
	}
}

func containerName(container model.Container) string {
	if len(container.Names) == 0 {
		return shortID(container.ID)
	}
	return strings.TrimPrefix(container.Names[0], "/")
}

func imageName(img model.Image) string {
	if len(img.RepoTags) > 0 {
		return img.RepoTags[0]
	}
	return shortID(img.ID)
}

func renderPorts(ports []model.Port) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		switch {
		case port.PublicPort > 0:
			parts = append(parts, fmt.Sprintf("%d:%d/%s", port.PublicPort, port.PrivatePort, port.Type))
		case port.PrivatePort > 0:
			parts = append(parts, fmt.Sprintf("%d/%s", port.PrivatePort, port.Type))
		}
	}
	return strings.Join(parts, ", ")
}

func formatUnix(ts int64) string {
	if ts == 0 {
		return "-"
	}
	return time.Unix(ts, 0).Local().Format("2006-01-02 15:04")
}

func humanSize(v int64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%d B", v)
	}
	div, exp := int64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(v)/float64(div), "KMGTPE"[exp])
}

func shortID(v string) string {
	v = strings.TrimPrefix(v, "sha256:")
	if len(v) > 12 {
		return v[:12]
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "-"
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
