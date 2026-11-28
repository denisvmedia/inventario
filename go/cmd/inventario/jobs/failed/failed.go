// Package failed implements `inventario jobs failed list|retry|discard`
// (#1307).
package failed

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"go.5x5.cz/inventario/cmd/internal/command"
	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/admin"
)

// New creates the `failed` command group.
func New(dbConfig *shared.DatabaseConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "failed",
		Short: "List, retry and discard background jobs that failed for good",
		Long: `List, retry and discard background jobs that failed for good.

Job types: ` + jobTypeList() + `

A restore and a currency migration cannot be retried: a restore may have
written part of its backup, and a migration ran on a rate confirmed for
that moment. Discard them and start a new one instead.`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newList(dbConfig).Cmd())
	cmd.AddCommand(newAction(dbConfig, "retry").Cmd())
	cmd.AddCommand(newAction(dbConfig, "discard").Cmd())
	return cmd
}

// openService checks the database configuration and opens the admin
// service. memory:// is refused first, with a message about why.
func openService(dbConfig *shared.DatabaseConfig) (*admin.Service, error) {
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return nil, errors.New("failed-job commands are not supported for memory databases: the jobs live in the database the workers share; use PostgreSQL")
	}
	if err := dbConfig.Validate(); err != nil {
		return nil, errxtrace.Wrap("database configuration error", err)
	}
	return admin.NewService(dbConfig)
}

func closeService(out io.Writer, svc *admin.Service) {
	if err := svc.Close(); err != nil {
		fmt.Fprintf(out, "Warning: failed to close admin service: %v\n", err)
	}
}

func jobTypeList() string {
	types := models.FailedJobTypes()
	names := make([]string, 0, len(types))
	for _, t := range types {
		name := string(t)
		if !models.FailedJobRetryable(t) {
			name += " (discard only)"
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// ListConfig carries the list command's flags.
type ListConfig struct {
	Type         string `yaml:"type" env:"TYPE"`
	Tenant       string `yaml:"tenant" env:"TENANT"`
	Limit        int    `yaml:"limit" env:"LIMIT"`
	FailedBefore string `yaml:"failed_before" env:"FAILED_BEFORE"`
}

type listCommand struct {
	command.Base

	config ListConfig
}

const defaultListLimit = 100

func newList(dbConfig *shared.DatabaseConfig) *listCommand {
	c := &listCommand{config: ListConfig{Limit: defaultListLimit}}
	shared.TryReadSection("jobs.failed.list", &c.config)
	c.Base = command.NewBase(&cobra.Command{
		Use:   "list",
		Short: "List the failed jobs nobody retried or discarded",
		Long: `List the background jobs that failed for good and were neither retried nor
discarded, newest failure first, across every tenant.

--failed-before lists the jobs that ` + "`inventario jobs failed discard --failed-before`" + `
with the same filter discards.

Examples:
  inventario jobs failed list
  inventario jobs failed list --type intake --tenant acme --limit 20
  inventario jobs failed list --failed-before 2026-11-01`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.run(cmd, dbConfig)
		},
	})
	flags := c.Cmd().Flags()
	flags.StringVar(&c.config.Type, "type", c.config.Type, "Only jobs of this type")
	flags.StringVar(&c.config.Tenant, "tenant", c.config.Tenant, "Only jobs of this tenant (id or slug)")
	flags.IntVar(&c.config.Limit, "limit", c.config.Limit, "Print at most this many jobs (0 prints all)")
	flags.StringVar(&c.config.FailedBefore, "failed-before", c.config.FailedBefore, failedBeforeUsage)
	return c
}

const failedBeforeUsage = "Only jobs that failed before this time: a date (midnight UTC) or an RFC 3339 time"

// parseFailedBeforeFlag reads --failed-before; empty is the zero time.
func parseFailedBeforeFlag(raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	t, err := services.ParseFailedBefore(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --failed-before %q: use a date such as 2026-11-01 or an RFC 3339 time: %w", raw, err)
	}
	return t, nil
}

func (c *listCommand) run(cmd *cobra.Command, dbConfig *shared.DatabaseConfig) error {
	out := cmd.OutOrStdout()
	failedBefore, err := parseFailedBeforeFlag(c.config.FailedBefore)
	if err != nil {
		return err
	}
	svc, err := openService(dbConfig)
	if err != nil {
		return err
	}
	defer closeService(out, svc)

	page, err := svc.ListFailedJobs(cmd.Context(), admin.FailedJobListRequest{
		Type:         c.config.Type,
		Tenant:       c.config.Tenant,
		Limit:        c.config.Limit,
		FailedBefore: failedBefore,
	})
	if err != nil {
		return errxtrace.Wrap("failed to list failed jobs", err)
	}
	if len(page.Jobs) == 0 {
		fmt.Fprintln(out, "No failed jobs.")
		return nil
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tID\tTENANT\tGROUP\tFAILED_AT\tATTEMPTS\tRETRYABLE\tTITLE\tERROR")
	for _, job := range page.Jobs {
		failedAt := "-"
		if job.FailedAt != nil {
			failedAt = job.FailedAt.UTC().Format("2006-01-02 15:04:05")
		}
		retryable := "no"
		if models.FailedJobRetryable(job.Type) {
			retryable = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			job.Type, job.ID, orDash(job.TenantID), orDash(job.GroupID), failedAt, job.Attempts, retryable,
			orDash(oneLine(job.Title)), orDash(oneLine(job.Error)))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if page.Total > len(page.Jobs) {
		fmt.Fprintf(out, "Showing %d of %d failed jobs. Use --limit 0 to print all.\n", len(page.Jobs), page.Total)
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// oneLine keeps a multi-line error on its table row. Control characters,
// an escape sequence's ESC among them, become spaces: a job's title and
// error come from the job's data, and the terminal must not act on them.
func oneLine(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// ActionConfig carries the retry and discard commands' flags. FailedBefore,
// Tenant and DryRun are the discard command's bulk form.
type ActionConfig struct {
	Type         string `yaml:"type" env:"TYPE"`
	FailedBefore string `yaml:"failed_before" env:"FAILED_BEFORE"`
	Tenant       string `yaml:"tenant" env:"TENANT"`
	DryRun       bool   `yaml:"dry_run" env:"DRY_RUN"`
}

type actionCommand struct {
	command.Base

	verb   string
	config ActionConfig
}

func newAction(dbConfig *shared.DatabaseConfig, verb string) *actionCommand {
	c := &actionCommand{verb: verb}
	shared.TryReadSection("jobs.failed."+verb, &c.config)
	short, long := "Put a failed job back in its queue", `Put a failed job back in its queue the way its type runs it again: an
export or import runs from the start, a personal data export builds its
copy again from the start and keeps it 7 days from when that build
finishes, a thumbnail gets a fresh set of attempts, an intake scan is due
at once. A restore and a currency migration cannot be retried; discard
them instead.

The retry is written to the audit log.

Example:
  inventario jobs failed retry --type export <job-id>`
	use, args := verb+" <job-id>", cobra.ExactArgs(1)
	if verb == "discard" {
		short, long = "Discard a failed job, or every job that failed before a time", `Discard a failed job. It leaves the failed list and the depth gauge, but
its row and its error stay until the normal cleanup removes them. Every
job type can be discarded.

Without a job id, --failed-before discards every failed job that failed
before the time, of the --type and --tenant given or of every type and
tenant. A job without a failure time counts from its creation. The time
is a date, which stands for its midnight UTC, or an RFC 3339 time, and
must not be in the future. --dry-run counts the jobs and discards
nothing; `+"`inventario jobs failed list`"+` with the same filter lists them.

A discard is written to the audit log: a bulk discard as one row that
names the filter and the counts.

Examples:
  inventario jobs failed discard --type restore <job-id>
  inventario jobs failed discard --failed-before 2026-11-01 --dry-run
  inventario jobs failed discard --type export --failed-before 2026-11-01`
		use, args = verb+" [<job-id>]", cobra.MaximumNArgs(1)
	}
	c.Base = command.NewBase(&cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  args,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return c.runBulk(cmd, dbConfig)
			}
			return c.run(cmd, dbConfig, args[0])
		},
	})
	flags := c.Cmd().Flags()
	if verb != "discard" {
		flags.StringVar(&c.config.Type, "type", c.config.Type, "Type of the job (required): "+jobTypeList())
		return c
	}
	flags.StringVar(&c.config.Type, "type", c.config.Type,
		"Type of the job (required with a job id; without one, only jobs of this type): "+jobTypeList())
	flags.StringVar(&c.config.FailedBefore, "failed-before", c.config.FailedBefore,
		"Without a job id: discard the jobs that failed before this time, a date (midnight UTC) or an RFC 3339 time")
	flags.StringVar(&c.config.Tenant, "tenant", c.config.Tenant, "With --failed-before: only jobs of this tenant (id or slug)")
	flags.BoolVar(&c.config.DryRun, "dry-run", c.config.DryRun, "With --failed-before: count the jobs and discard nothing")
	return c
}

// runBulk discards every failed job that failed before --failed-before.
func (c *actionCommand) runBulk(cmd *cobra.Command, dbConfig *shared.DatabaseConfig) error {
	out := cmd.OutOrStdout()
	if strings.TrimSpace(c.config.FailedBefore) == "" {
		return errors.New("give a job id, or --failed-before to discard every job that failed before a time")
	}
	failedBefore, err := parseFailedBeforeFlag(c.config.FailedBefore)
	if err != nil {
		return err
	}
	svc, err := openService(dbConfig)
	if err != nil {
		return err
	}
	defer closeService(out, svc)

	counts, err := svc.DiscardFailedJobsBefore(cmd.Context(), admin.FailedJobBulkDiscardRequest{
		Type:         c.config.Type,
		Tenant:       c.config.Tenant,
		FailedBefore: failedBefore,
		DryRun:       c.config.DryRun,
	})
	cutoff := failedBefore.UTC().Format(time.RFC3339)
	switch {
	case errors.Is(err, registry.ErrUnknownFailedJobType):
		return fmt.Errorf("unknown job type %q; known types: %s: %w", c.config.Type, jobTypeList(), err)
	case errors.Is(err, registry.ErrInvalidInput):
		return fmt.Errorf("--failed-before %s is in the future: %w", cutoff, err)
	case err != nil && counts.Total > 0:
		return errxtrace.Wrap(fmt.Sprintf("discarded %s (%s), then failed", failedJobs(counts.Total), countsByType(counts)), err)
	case err != nil:
		return errxtrace.Wrap("failed to discard the jobs", err)
	}
	switch {
	case counts.Total == 0:
		fmt.Fprintf(out, "No failed jobs failed before %s.\n", cutoff)
	case c.config.DryRun:
		fmt.Fprintf(out, "Would discard %s that failed before %s: %s. Nothing was discarded.\n", failedJobs(counts.Total), cutoff, countsByType(counts))
	default:
		fmt.Fprintf(out, "Discarded %s that failed before %s: %s.\n", failedJobs(counts.Total), cutoff, countsByType(counts))
	}
	return nil
}

func failedJobs(n int) string {
	if n == 1 {
		return "1 failed job"
	}
	return fmt.Sprintf("%d failed jobs", n)
}

// countsByType prints the counts per type, in the order of the job types.
func countsByType(counts services.FailedJobCounts) string {
	parts := make([]string, 0, len(counts.ByType))
	for _, t := range models.FailedJobTypes() {
		if n := counts.ByType[string(t)]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", t, n))
		}
	}
	return strings.Join(parts, ", ")
}

func (c *actionCommand) run(cmd *cobra.Command, dbConfig *shared.DatabaseConfig, id string) error {
	out := cmd.OutOrStdout()
	jobType := strings.TrimSpace(c.config.Type)
	if jobType == "" {
		return errors.New("--type is required")
	}
	if c.config.FailedBefore != "" || c.config.Tenant != "" || c.config.DryRun {
		return errors.New("--failed-before, --tenant and --dry-run discard in bulk; leave out the job id")
	}
	// An unknown type is refused by the service, which audits the refusal
	// like any other (#1307).
	svc, err := openService(dbConfig)
	if err != nil {
		return err
	}
	defer closeService(out, svc)

	act, done := svc.RetryFailedJob, "Retried"
	if c.verb == "discard" {
		act, done = svc.DiscardFailedJob, "Discarded"
	}
	job, err := act(cmd.Context(), jobType, strings.TrimSpace(id))
	if errors.Is(err, registry.ErrUnknownFailedJobType) {
		return fmt.Errorf("unknown job type %q; known types: %s: %w", jobType, jobTypeList(), err)
	}
	if err != nil {
		return errxtrace.Wrap("failed to "+c.verb+" the job", err)
	}
	fmt.Fprintf(out, "%s %s job %s (tenant %s).\n", done, job.Type, job.ID, orDash(job.TenantID))
	return nil
}
