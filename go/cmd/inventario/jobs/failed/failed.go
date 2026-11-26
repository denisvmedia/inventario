// Package failed implements `inventario jobs failed list|retry|discard`
// (#1307).
package failed

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"unicode"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"go.5x5.cz/inventario/cmd/internal/command"
	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
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
	Type   string `yaml:"type" env:"TYPE"`
	Tenant string `yaml:"tenant" env:"TENANT"`
	Limit  int    `yaml:"limit" env:"LIMIT"`
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

Examples:
  inventario jobs failed list
  inventario jobs failed list --type intake --tenant acme --limit 20`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.run(cmd, dbConfig)
		},
	})
	flags := c.Cmd().Flags()
	flags.StringVar(&c.config.Type, "type", c.config.Type, "Only jobs of this type")
	flags.StringVar(&c.config.Tenant, "tenant", c.config.Tenant, "Only jobs of this tenant (id or slug)")
	flags.IntVar(&c.config.Limit, "limit", c.config.Limit, "Print at most this many jobs (0 prints all)")
	return c
}

func (c *listCommand) run(cmd *cobra.Command, dbConfig *shared.DatabaseConfig) error {
	out := cmd.OutOrStdout()
	svc, err := openService(dbConfig)
	if err != nil {
		return err
	}
	defer closeService(out, svc)

	page, err := svc.ListFailedJobs(cmd.Context(), admin.FailedJobListRequest{
		Type:   c.config.Type,
		Tenant: c.config.Tenant,
		Limit:  c.config.Limit,
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

// ActionConfig carries the retry and discard commands' flags.
type ActionConfig struct {
	Type string `yaml:"type" env:"TYPE"`
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
export or import runs from the start, a thumbnail gets a fresh set of
attempts, an intake scan is due at once. A restore and a currency
migration cannot be retried; discard them instead.

The retry is written to the audit log.

Example:
  inventario jobs failed retry --type export <job-id>`
	if verb == "discard" {
		short, long = "Discard a failed job", `Discard a failed job. It leaves the failed list and the depth gauge, but
its row and its error stay until the normal cleanup removes them. Every
job type can be discarded.

The discard is written to the audit log.

Example:
  inventario jobs failed discard --type restore <job-id>`
	}
	c.Base = command.NewBase(&cobra.Command{
		Use:   verb + " <job-id>",
		Short: short,
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.run(cmd, dbConfig, args[0])
		},
	})
	c.Cmd().Flags().StringVar(&c.config.Type, "type", c.config.Type, "Type of the job (required): "+jobTypeList())
	return c
}

func (c *actionCommand) run(cmd *cobra.Command, dbConfig *shared.DatabaseConfig, id string) error {
	out := cmd.OutOrStdout()
	jobType := strings.TrimSpace(c.config.Type)
	if jobType == "" {
		return errors.New("--type is required")
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
