package cli

import (
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

// Gated (--yes) project-domain mutations, split from domain.go to mirror the
// deployment.go / deployment_write.go read-vs-write convention. The DNS
// `records` sub-group (which mixes a read with its own add/rm) stays in
// domain.go as a cohesive unit.

func domainAddCmd(g *GlobalFlags) *cobra.Command {
	var redirect, gitBranch string
	var yes *bool
	cmd := &cobra.Command{
		Use:   "add <project> <domain>",
		Short: "Add a domain to a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, domain := args[0], args[1]
			r, err := confirmAndClient(g, *yes, "add domain "+domain+" to "+project,
				"agent-vercel domain add "+project+" "+domain+" --yes")
			if err != nil {
				return err
			}
			body := map[string]any{"name": domain}
			putIf(body, "redirect", redirect)
			putIf(body, "gitBranch", gitBranch)
			raw, err := r.client.AddProjectDomain(cmd.Context(), project, body)
			if err != nil {
				return err
			}
			return printRaw(g, raw)
		},
	}
	cmd.Flags().StringVar(&redirect, "redirect", "", "redirect target domain")
	cmd.Flags().StringVar(&gitBranch, "git-branch", "", "git branch to link the domain to")
	yes = addYesFlag(cmd)
	return cmd
}

func domainRmCmd(g *GlobalFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "rm <project> <domain>",
		Short: "Remove a domain from a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, domain := args[0], args[1]
			r, err := confirmAndClient(g, *yes, "remove domain "+domain+" from "+project,
				"agent-vercel domain rm "+project+" "+domain+" --yes")
			if err != nil {
				return err
			}
			if _, err := r.client.RemoveProjectDomain(cmd.Context(), project, domain); err != nil {
				return err
			}
			return printSingle(g, map[string]any{"removed": domain, "project": project})
		},
	}
	yes = addYesFlag(cmd)
	return cmd
}

func domainVerifyCmd(g *GlobalFlags) *cobra.Command {
	var project string
	var yes *bool
	cmd := &cobra.Command{
		Use:   "verify <domain> --project <p>",
		Short: "Trigger verification of a project domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			if project == "" {
				return agenterrors.New("--project is required", agenterrors.FixableByAgent).
					WithHint("pass --project <id|name>")
			}
			r, err := confirmAndClient(g, *yes, "verify domain "+domain+" on "+project,
				"agent-vercel domain verify "+domain+" --project "+project+" --yes")
			if err != nil {
				return err
			}
			raw, err := r.client.VerifyProjectDomain(cmd.Context(), project, domain)
			if err != nil {
				return err
			}
			return printRaw(g, raw)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project id or name (required)")
	yes = addYesFlag(cmd)
	return cmd
}
