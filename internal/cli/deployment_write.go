package cli

import (
	"context"
	"encoding/json"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/vercel"
	"github.com/spf13/cobra"
)

// deploymentRef resolves a deployment id/url to its canonical id, project id,
// name, and target — needed by the project-scoped write endpoints. It reads the
// typed deployment shape directly (the same fields compactDeployment projects),
// so the write commands depend on the API contract rather than the output
// projection.
func deploymentRef(ctx context.Context, client *vercel.Client, idOrURL string) (id, projectID, name, target string, err error) {
	raw, err := client.GetDeployment(ctx, cleanRef(idOrURL))
	if err != nil {
		return "", "", "", "", err
	}
	var d rawDeployment
	if err := json.Unmarshal(raw, &d); err != nil {
		return "", "", "", "", agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}
	id = firstNonEmpty(d.UID, d.ID)
	projectID = d.ProjectID
	name = d.Name
	if d.Target != nil {
		target = *d.Target
	}
	if id == "" || projectID == "" {
		return "", "", "", "", agenterrors.New("could not resolve deployment", agenterrors.FixableByAgent)
	}
	return id, projectID, name, target, nil
}

func deploymentPromoteCmd(g *GlobalFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "promote <id|url>",
		Short: "Promote a deployment to production (repoints traffic; no rebuild)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := confirmAndClient(g, *yes, "promote "+args[0]+" to production", "agent-vercel deployment promote "+args[0]+" --yes")
			if err != nil {
				return err
			}
			id, projectID, _, _, err := deploymentRef(cmd.Context(), r.client, args[0])
			if err != nil {
				return err
			}
			if _, err := r.client.PromoteDeployment(cmd.Context(), projectID, id); err != nil {
				return err
			}
			return printSingle(g, map[string]any{"promoted": id, "project": projectID})
		},
	}
	yes = addYesFlag(cmd)
	return cmd
}

func deploymentRollbackCmd(g *GlobalFlags) *cobra.Command {
	var yes *bool
	var description string
	cmd := &cobra.Command{
		Use:   "rollback <id|url>",
		Short: "Roll production back to a previous deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := confirmAndClient(g, *yes, "roll production back to "+args[0], "agent-vercel deployment rollback "+args[0]+" --yes")
			if err != nil {
				return err
			}
			id, projectID, _, _, err := deploymentRef(cmd.Context(), r.client, args[0])
			if err != nil {
				return err
			}
			if _, err := r.client.RollbackDeployment(cmd.Context(), projectID, id, description); err != nil {
				return err
			}
			return printSingle(g, map[string]any{"rolled_back_to": id, "project": projectID})
		},
	}
	yes = addYesFlag(cmd)
	cmd.Flags().StringVar(&description, "description", "", "reason for the rollback")
	return cmd
}

func deploymentCancelCmd(g *GlobalFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "cancel <id|url>",
		Short: "Cancel an in-progress deployment build",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := confirmAndClient(g, *yes, "cancel build "+args[0], "agent-vercel deployment cancel "+args[0]+" --yes")
			if err != nil {
				return err
			}
			id, _, _, _, err := deploymentRef(cmd.Context(), r.client, args[0])
			if err != nil {
				return err
			}
			if _, err := r.client.CancelDeployment(cmd.Context(), id); err != nil {
				return err
			}
			return printSingle(g, map[string]any{"canceled": id})
		},
	}
	yes = addYesFlag(cmd)
	return cmd
}

func deploymentRedeployCmd(g *GlobalFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "redeploy <id|url>",
		Short: "Rebuild and deploy a new copy of an existing deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := confirmAndClient(g, *yes, "redeploy "+args[0], "agent-vercel deployment redeploy "+args[0]+" --yes")
			if err != nil {
				return err
			}
			id, _, name, target, err := deploymentRef(cmd.Context(), r.client, args[0])
			if err != nil {
				return err
			}
			body := map[string]any{"deploymentId": id, "name": name}
			if target != "" {
				body["target"] = target
			}
			raw, err := r.client.CreateDeployment(cmd.Context(), body)
			if err != nil {
				return err
			}
			return getOne(g, raw, compactDeployment)
		},
	}
	yes = addYesFlag(cmd)
	return cmd
}
