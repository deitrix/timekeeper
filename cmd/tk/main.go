package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
)

var grey = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("246"))

var white = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("15"))

var cyan = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("14"))

var green = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("10"))

var red = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("9"))

func main() {
	godotenv.Load()

	ctx, cfn := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cfn()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	db, err := readDB()
	if err != nil {
		return fmt.Errorf("read db: %w", err)
	}

	// Clone the DB so that we can compare it later to see if it changed.
	a := &App{DB: db.Clone()}

	rootCmd := a.createRootCmd()
	if err := rootCmd.Run(ctx, os.Args); err != nil {
		return err
	}

	if !db.Equal(a.DB) {
		if err := writeDB(a.DB); err != nil {
			return fmt.Errorf("write db: %w", err)
		}
	}

	return nil
}

func (a *App) createRootCmd() *cli.Command {
	return &cli.Command{
		Name: "tk",
		Commands: []*cli.Command{
			a.newCmd(),
			a.startCmd(),
			a.stopCmd(),
			a.startStopCmd(),
			a.listCmd(),
			a.archiveCmd(),
			a.removeCmd(),
			a.weekCmd(),
		},
		Action: func(ctx context.Context, command *cli.Command) error {
			if len(a.DB.Projects) == 0 {
				fmt.Println("No projects. Start one with `tk start <name>`")
				return nil
			}
			renderCurrent(a.DB.Projects[0])
			return nil
		},
	}
}

func (a *App) newCmd() *cli.Command {
	return &cli.Command{
		Name:  "new",
		Usage: "Create a new project",
		Action: func(ctx context.Context, command *cli.Command) error {
			name := command.Args().First()
			if name == "" {
				return errors.New("missing project name")
			}

			p := a.CreateProject(name)
			fmt.Printf("Created %s %s\n", p.Name, p.prettyRefParen())
			return nil
		},
	}
}

func (a *App) stopCmd() *cli.Command {
	return &cli.Command{
		Name:  "stop",
		Usage: "Stop the current project",
		Action: func(ctx context.Context, command *cli.Command) error {
			p, ok := a.InProgressProject()
			if !ok {
				fmt.Println("No project in progress")
				return nil
			}

			a.Stop(p)
			renderStopped(p)

			return nil
		},
	}
}

func (a *App) startCmd() *cli.Command {
	return &cli.Command{
		Name:  "start",
		Usage: "Start the current project, or create a new one",
		Action: func(ctx context.Context, command *cli.Command) error {
			name := command.Args().First()
			ref, err := strconv.Atoi(name)
			if err == nil {
				name = ""
			}

			p, err := a.GetOrCreateProject(ref, name)
			if err != nil {
				return err
			}

			// Stop the currently in-progress project, if any. There can only ever be at most one project in
			// progress at a time. So, this could well be the same project as the one we're about to start.
			if ip, ok := a.InProgressProject(); ok {
				if ip.ID == p.ID {
					fmt.Println("Project already in progress")
					return nil
				}

				a.Stop(ip)
				renderStopped(ip)
				fmt.Println()
			}

			a.Start(p)
			renderStarted(p)

			return nil
		},
	}
}

func (a *App) startStopCmd() *cli.Command {
	return &cli.Command{
		Name:  "s",
		Usage: "Context-aware start/stop",
		Action: func(ctx context.Context, command *cli.Command) error {
			name := command.Args().First()
			ref, err := strconv.Atoi(name)
			if err == nil {
				name = ""
			}

			p, err := a.GetOrCreateProject(ref, name)
			if err != nil {
				return err
			}

			// Stop the currently in-progress project, if any. There can only ever be at most one project in
			// progress at a time. So, this could well be the same project as the one we're about to start.
			if ip, ok := a.InProgressProject(); ok && a.Stop(ip) {
				renderStopped(ip)
				if p.ID == ip.ID {
					return nil
				}
				fmt.Println()
			}

			a.Start(p)
			renderStarted(p)

			return nil
		},
	}
}

func (a *App) listCmd() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls", "l"},
		Usage:   "List projects",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "List all projects",
			},
			&cli.BoolFlag{
				Name:    "all-archived",
				Aliases: []string{"A"},
				Usage:   "List all projects, including archived",
			},
			&cli.IntFlag{
				Name:  "n",
				Value: 15,
				Usage: "List the first n projects",
			},
		},
		Action: func(ctx context.Context, command *cli.Command) error {
			all := command.Bool("all")
			allArchived := command.Bool("all-archived")
			n := command.Int("n")
			projects := a.DB.ListProjects(allArchived)
			if len(projects) == 0 {
				fmt.Println("No projects")
				return nil
			}
			header := []string{"Ref", "Name", "Last Start", "Last Duration", "This Week", "Total"}
			if allArchived {
				header = append(header, "Archived")
			}
			for i, h := range header {
				header[i] = white.Render(h)
			}
			rows := [][]string{header}
			for _, p := range projects {
				if p.Archived {
					rows = append(rows, []string{
						p.prettyRef(),
						grey.Render(p.Name),
						grey.Render(p.LastStartFormatted()),
						grey.Render(p.DurationFormatted()),
						grey.Render(p.ThisWeekFormatted()),
						grey.Render(p.TotalFormatted()),
						grey.Render("True"),
					})
				} else {
					name := p.Name
					if p.InProgress() {
						name = green.Render(name)
					}
					rows = append(rows, []string{
						p.prettyRef(),
						name,
						grey.Render(p.LastStartFormatted()),
						grey.Render(p.DurationFormatted()),
						grey.Render(p.ThisWeekFormatted()),
						grey.Render(p.TotalFormatted()),
					})
				}
				if !all && !allArchived && len(rows) > int(n) {
					break
				}
			}

			fmt.Println(grid(rows...))
			return nil
		},
	}
}

func (a *App) archiveCmd() *cli.Command {
	return &cli.Command{
		Name:    "archive",
		Aliases: []string{"a"},
		Usage:   "Archive (or unarchive) a project",
		Action: func(ctx context.Context, command *cli.Command) error {
			var refs []int
			if command.NArg() == 0 {
				refs = []int{0}
			} else {
				for _, arg := range command.Args().Slice() {
					ref, err := strconv.Atoi(arg)
					if err != nil {
						return err
					}
					refs = append(refs, ref)
				}
			}

			for _, ref := range refs {
				p, err := a.ProjectByRef(ref)
				if err != nil {
					return err
				}

				if !p.Archived && p.InProgress() {
					a.Stop(p)
					renderStopped(p)
					fmt.Println()
				}

				p.Archived = !p.Archived
				if p.Archived {
					fmt.Printf("%s %s %s\n", grey.Render("Archived:"), p.Name, p.prettyRefParen())
				} else {
					fmt.Printf("%s %s %s\n", white.Render("Unarchived:"), p.Name, p.prettyRefParen())
				}
			}

			return nil
		},
	}
}

func (a *App) removeCmd() *cli.Command {
	return &cli.Command{
		Name:    "remove",
		Aliases: []string{"rm", "r"},
		Usage:   "Remove a project",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Remove all projects",
			},
		},
		Action: func(ctx context.Context, command *cli.Command) error {
			var refs []int
			switch {
			case command.Bool("all"):
				refs = make([]int, len(a.DB.Projects))
				for i, p := range a.DB.Projects {
					refs[i] = p.ID
				}
			case command.NArg() == 0:
				refs = []int{0}
			default:
				for _, arg := range command.Args().Slice() {
					ref, err := strconv.Atoi(arg)
					if err != nil {
						return err
					}
					refs = append(refs, ref)
				}
			}

			for _, ref := range refs {
				p, err := a.ProjectByRef(ref)
				if err != nil {
					return err
				}

				a.DB.RemoveProject(p)
				fmt.Printf("%s %s %s\n", red.Render("Removed:"), p.Name, p.prettyRefParen())
			}

			return nil
		},
	}
}

func (a *App) weekCmd() *cli.Command {
	return &cli.Command{
		Name:  "week",
		Usage: "Show a summary of the current week",
		Action: func(ctx context.Context, command *cli.Command) error {
			projects := a.DB.ListProjects(false)
			if len(projects) == 0 {
				fmt.Println("No projects")
				return nil
			}

			header := []string{"Name", "This Week", "Total"}
			for i, h := range header {
				header[i] = white.Render(h)
			}
			rows := [][]string{header}
			for _, p := range projects {
				thisWeek := p.ThisWeek()
				if thisWeek == 0 {
					continue
				}
				rows = append(rows, []string{
					p.Name,
					cyan.Render(formatDuration(thisWeek)),
					cyan.Render(p.TotalFormatted()),
				})
			}

			fmt.Println(grid(rows...))
			return nil
		},
	}
}

type App struct {
	DB DB
}

func renderStopped(p *Project) {
	fmt.Printf("%s %s %s\n", red.Render("Stopped:"), p.Name, p.prettyRefParen())
	fmt.Println()
	renderStats(p, true)
}

func renderStarted(p *Project) {
	fmt.Printf("%s %s %s\n", green.Render("Started:"), p.Name, p.prettyRefParen())
	if !p.JustCreated {
		fmt.Println()
		renderStats(p, false)
	}
}

func renderCurrent(p *Project) {
	var state string
	if p.InProgress() {
		state = green.Render("In progress:")
	} else {
		state = red.Render("Stopped:")
	}
	fmt.Printf("%s %s %s\n", state, p.Name, p.prettyRefParen())
	fmt.Println()
	renderStats(p, true)
}

func renderStats(p *Project, duration bool) {
	var rows [][]string
	if duration {
		rows = append(rows, []string{"Duration", p.DurationFormatted()})
	}
	rows = append(rows,
		[]string{"This week", p.ThisWeekFormatted()},
		[]string{"Total", p.TotalFormatted()},
	)
	fmt.Println(grid(rows...))
}

func (a *App) GetOrCreateProject(ref int, newProjectName string) (*Project, error) {
	if newProjectName != "" {
		return a.CreateProject(newProjectName), nil
	}
	p, err := a.ProjectByRef(ref)
	if err == nil {
		return p, nil
	}
	return nil, err
}

func (a *App) CreateProject(name string) *Project {
	p := &Project{Name: name, JustCreated: true}
	a.DB.CreateProject(p)
	return p
}

func (a *App) Stop(p *Project) bool {
	e, ok := p.LastEntry()
	if !ok {
		return false
	}
	e.End = time.Now()
	p.Entries[len(p.Entries)-1] = e
	return true
}

func (a *App) Start(p *Project) {
	p.Entries = append(p.Entries, Entry{Start: time.Now()})
}

func (a *App) InProgressProject() (*Project, bool) {
	for _, p := range a.DB.Projects {
		if p.InProgress() {
			return p, true
		}
	}
	return nil, false
}

func (a *App) ProjectByRef(ref int) (*Project, error) {
	if len(a.DB.Projects) == 0 {
		return nil, ErrNoProjects
	}
	for _, p := range a.DB.Projects {
		if p.Ref == ref || p.ID == ref {
			return p, nil
		}
	}
	return nil, errors.New("project not found")
}

var ErrNoProjects = errors.New("no projects")

func getDBPath() (string, error) {
	if path := os.Getenv("TIMEKEEPER_DB"); path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".timekeeper", "db.json"), nil
}

type Project struct {
	ID          int       `json:"id"`
	Ref         int       `json:"ref"`
	Name        string    `json:"name"`
	Entries     []Entry   `json:"entries"`
	Archived    bool      `json:"archived"`
	JustCreated bool      `json:"-"`
	Created     time.Time `json:"created"`
}

func (p Project) prettyRef() string {
	if p.Ref == p.ID {
		return cyan.Render(strconv.Itoa(p.Ref))
	}
	return fmt.Sprintf("%s (id=%s)", cyan.Render(strconv.Itoa(p.Ref)), cyan.Render(strconv.Itoa(p.ID)))
}

func (p *Project) prettyRefParen() string {
	if p.Ref == p.ID {
		return fmt.Sprintf("(ref=%s)", cyan.Render(strconv.Itoa(p.Ref)))
	}
	if p.JustCreated {
		return fmt.Sprintf("(id=%s)", cyan.Render(strconv.Itoa(p.ID)))
	}
	return fmt.Sprintf("(ref=%s id=%s)", cyan.Render(strconv.Itoa(p.Ref)), cyan.Render(strconv.Itoa(p.ID)))
}

func (p *Project) Equal(other *Project) bool {
	if p == nil || other == nil {
		return p == other
	}
	return p.ID == other.ID &&
		p.Name == other.Name &&
		p.Archived == other.Archived &&
		slices.Equal(p.Entries, other.Entries)
}

func (p Project) Clone() *Project {
	p.Entries = slices.Clone(p.Entries)
	return &p
}

func (p *Project) InProgress() bool {
	e, ok := p.LastEntry()
	return ok && e.InProgress()
}

func (p *Project) LastEntry() (Entry, bool) {
	if len(p.Entries) == 0 {
		return Entry{}, false
	}
	return p.Entries[len(p.Entries)-1], true
}

func (p Project) DurationFormatted() string {
	e, ok := p.LastEntry()
	if !ok {
		return cyan.Render("-")
	}
	return cyan.Render(formatDuration(e.Duration()))
}

func (p Project) ThisWeek() time.Duration {
	_, week := time.Now().ISOWeek()

	var total time.Duration
	for _, e := range p.Entries {
		_, w := e.Start.ISOWeek()
		if w == week {
			total += e.Duration()
		}
	}

	return total
}

func (p Project) ThisWeekFormatted() string {
	if len(p.Entries) == 0 {
		return cyan.Render("-")
	}
	return cyan.Render(formatDuration(p.ThisWeek()))
}

func (p Project) Total() time.Duration {
	var total time.Duration
	for _, e := range p.Entries {
		total += e.Duration()
	}
	return total
}

func (p Project) TotalFormatted() string {
	if len(p.Entries) == 0 {
		return cyan.Render("-")
	}
	return cyan.Render(formatDuration(p.Total()))
}

func (p Project) LastStartFormatted() string {
	e, ok := p.LastEntry()
	if !ok {
		return cyan.Render("-")
	}
	return cyan.Render(formatDuration(time.Since(e.Start)) + " ago")
}

type Entry struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func (e Entry) Duration() time.Duration {
	if e.End.IsZero() {
		return time.Now().Sub(e.Start)
	}
	return e.End.Sub(e.Start)
}

func (e Entry) InProgress() bool {
	return e.End.IsZero()
}

type DB struct {
	ProjectID int        `json:"-"`
	Projects  []*Project `json:"projects"`
}

func (db *DB) Init() {

	// Sort projects so that most recently started projects are first, and archived projects are last.
	slices.SortFunc(db.Projects, (*Project).Compare)

	// Initially, give each project a reference number equal to its ID. Also, update the project ID
	// to be the highest ID in the list.
	db.ProjectID = 9
	for _, p := range db.Projects {
		p.Ref = p.ID
		db.ProjectID = max(db.ProjectID, p.ID)
	}

	// For the first 10 projects, give them a reference number equal to their index in the list.
	for i, p := range db.ListProjects(false) {
		if i >= 10 {
			break
		}
		p.Ref = i
	}

}

func (db DB) Equal(other DB) bool {
	return db.ProjectID == other.ProjectID &&
		slices.EqualFunc(db.Projects, other.Projects, (*Project).Equal)
}

func (db DB) Clone() DB {
	cloned := make([]*Project, len(db.Projects))
	for i, p := range db.Projects {
		cloned[i] = p.Clone()
	}
	db.Projects = cloned
	return db
}

func (db *DB) ListProjects(all bool) []*Project {
	if all {
		return db.Projects
	}

	// Filter out archived projects
	var active []*Project
	for _, p := range db.Projects {
		if !p.Archived {
			active = append(active, p)
		}
	}

	return active
}

func (db *DB) CreateProject(p *Project) {
	db.ProjectID++
	p.ID = db.ProjectID
	p.Created = time.Now()
	db.Projects = append(db.Projects, p)
}

func (db *DB) RemoveProject(p *Project) {
	var projects []*Project
	for _, project := range db.Projects {
		if project.ID != p.ID {
			projects = append(projects, project)
		}
	}
	db.Projects = projects
}

// byMostRecent sorts projects such that:
// - projects with no entries are last
// - projects with entries are sorted in descending order by the start time of the last entry
// - archived projects are last
// - projects with no entries are sorted by creation time in descending order
func (p *Project) Compare(p2 *Project) int {
	if p.Archived != p2.Archived {
		if p.Archived {
			return 1
		}
		return -1
	}
	e1, ok1 := p.LastEntry()
	e2, ok2 := p2.LastEntry()

	if !ok1 && !ok2 {
		return p2.Created.Compare(p.Created)
	}
	if !ok1 {
		return 1
	}
	if !ok2 {
		return -1
	}

	return e2.Start.Compare(e1.Start)
}

func readDB() (DB, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return DB{}, err
	}

	f, err := os.Open(dbPath)
	if errors.Is(err, os.ErrNotExist) {
		return DB{}, nil
	}
	if err != nil {
		return DB{}, err
	}
	defer f.Close()

	var db DB
	if err := json.NewDecoder(f).Decode(&db); err != nil {
		return DB{}, err
	}

	db.Init()

	return db, nil
}

func writeDB(db DB) error {
	dbPath, err := getDBPath()
	if err != nil {
		return err
	}

	f, err := os.Create(dbPath)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return err
		}
		f, err = os.Create(dbPath)
	}
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(db); err != nil {
		return err
	}

	return nil
}

func grid(rows ...[]string) string {
	t := table.New().
		Headers(rows[0]...).
		Rows(rows[1:]...).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderRow(false).
		BorderHeader(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if col == 0 {
				return lipgloss.NewStyle()
			}
			return lipgloss.NewStyle().PaddingLeft(2)
		})

	return t.String()
}

func formatDuration(dur time.Duration) string {
	var parts []string
	if d := int(dur.Hours()); d > 24 {
		parts = append(parts, fmt.Sprintf("%dd", d/24))
		dur -= time.Duration(d) * 24 * time.Hour
	}
	if h := int(dur.Hours()); h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
		dur -= time.Duration(h) * time.Hour
	}
	if m := int(dur.Minutes()); m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
		dur -= time.Duration(m) * time.Minute
	}
	if s := int(dur.Seconds()); s > 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	if len(parts) == 0 {
		return "-"
	}
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, "")
}
