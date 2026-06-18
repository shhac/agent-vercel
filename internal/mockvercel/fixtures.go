package mockvercel

// User and Team are the fixture shapes. Exported so callers can override them.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

type Team struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Options configures the fixtures served by the handler.
type Options struct {
	User               User
	Teams              []Team
	TeamMembers        []map[string]any
	Deployments        []map[string]any
	Projects           []map[string]any
	RollingRelease     map[string]any
	ProjectCrons       map[string]any
	CustomEnvironments []map[string]any
	DeploymentChecks   []map[string]any
	BuildEvents        []map[string]any
	RuntimeLogs        []map[string]any
	Env                []map[string]any
	SharedEnv          []map[string]any
	Domains            []map[string]any
	DomainConfig       map[string]any
	DomainRecords      []map[string]any
	Certs              map[string]map[string]any
	Aliases            []map[string]any
	Charges            []map[string]any
	Webhooks           []map[string]any
	EdgeConfigs        []map[string]any
	EdgeConfigItems    map[string][]map[string]any
	FirewallConfig     map[string]any
	AttackStatus       map[string]any
	FirewallBypass     map[string]any
	Drains             []map[string]any
	// RuntimeLogsHang, when set, makes the runtime-logs handler hold the
	// connection open after emitting its lines — simulating Vercel's
	// open-ended stream so the client's bounded-window read can be tested.
	RuntimeLogsHang bool
}

// Option mutates Options.
type Option func(*Options)

// WithEnv overrides the fixture environment variables.
func WithEnv(env []map[string]any) Option { return func(o *Options) { o.Env = env } }

// WithDeployments overrides the fixture deployments.
func WithDeployments(d []map[string]any) Option { return func(o *Options) { o.Deployments = d } }

// WithDeploymentChecks overrides the fixture deployment checks.
func WithDeploymentChecks(c []map[string]any) Option {
	return func(o *Options) { o.DeploymentChecks = c }
}

// WithProjectCrons overrides the fixture project crons payload.
func WithProjectCrons(c map[string]any) Option { return func(o *Options) { o.ProjectCrons = c } }

// WithCustomEnvironments overrides the fixture custom environments.
func WithCustomEnvironments(e []map[string]any) Option {
	return func(o *Options) { o.CustomEnvironments = e }
}

// WithWebhooks overrides the fixture webhooks.
func WithWebhooks(w []map[string]any) Option { return func(o *Options) { o.Webhooks = w } }

// WithRuntimeLogsHang makes the runtime-logs endpoint hold the connection open
// after emitting its lines, simulating Vercel's open-ended log stream.
func WithRuntimeLogsHang() Option { return func(o *Options) { o.RuntimeLogsHang = true } }

func defaults() *Options {
	return &Options{
		User: User{ID: "usr_mock", Username: "acme-bot", Email: "bot@acme.com", Name: "Acme Bot"},
		Teams: []Team{
			{ID: "team_abc", Slug: "acme", Name: "Acme Inc"},
			{ID: "team_xyz", Slug: "side", Name: "Side Project"},
		},
		TeamMembers: []map[string]any{
			{"uid": "usr_owner", "username": "acme-bot", "email": "bot@acme.com", "role": "OWNER", "confirmed": true, "createdAt": int64(1700000000000)},
			{"uid": "usr_dev", "username": "dev", "email": "dev@acme.com", "role": "MEMBER", "confirmed": true, "createdAt": int64(1710000000000)},
			{"uid": "usr_pending", "username": "newbie", "email": "newbie@acme.com", "role": "MEMBER", "confirmed": false, "createdAt": int64(1716000000000)},
		},
		Deployments: []map[string]any{
			{
				"uid": "dpl_ready", "name": "web", "projectId": "prj_web",
				"url": "web-ready.vercel.app", "state": "READY", "readyState": "READY",
				"target": "production", "readySubstate": "PROMOTED",
				"inspectorUrl": "https://vercel.com/acme/web/ready", "created": int64(1716206800000),
				"creator": map[string]any{"username": "acme-bot", "email": "bot@acme.com"},
				"meta":    map[string]any{"githubCommitRef": "main", "githubCommitSha": "abc123", "githubCommitMessage": "ship it"},
			},
			{
				"uid": "dpl_err", "name": "web", "projectId": "prj_web",
				"url": "web-err.vercel.app", "state": "ERROR", "readyState": "ERROR",
				"target": "production", "errorCode": "BUILD_FAILED",
				"errorMessage": "Command \"next build\" exited with 1",
				"inspectorUrl": "https://vercel.com/acme/web/err", "created": int64(1716206500000),
				"checksConclusion": "failed", "oomReport": "out-of-memory",
				"creator": map[string]any{"username": "dev", "email": "dev@acme.com"},
				"meta":    map[string]any{"githubCommitRef": "fix/build", "githubCommitSha": "def456", "githubCommitMessage": "wip"},
			},
		},
		Projects: []map[string]any{
			{
				"id": "prj_web", "name": "web", "framework": "nextjs", "nodeVersion": "20.x",
				"rootDirectory":  "apps/web", "outputDirectory": ".next",
				"buildCommand":   "turbo run build", "installCommand": nil,
				"commandForIgnoringBuildStep": "npx turbo-ignore",
				"link":      map[string]any{"org": "acme", "repo": "web", "type": "github", "productionBranch": "main"},
				"updatedAt": int64(1716206800000),
				"latestDeployments": []any{map[string]any{
					"uid": "dpl_ready", "url": "web-ready.vercel.app", "readyState": "READY", "target": "production",
				}},
			},
			{
				"id": "prj_api", "name": "api", "framework": "go", "paused": true,
				"updatedAt": int64(1716100000000),
			},
		},
		RollingRelease: map[string]any{
			"rollingRelease": map[string]any{
				"state": "ACTIVE",
				"currentDeployment": map[string]any{
					"id": "dpl_ready", "url": "web-ready.vercel.app", "readyState": "READY", "target": "production",
				},
				"canaryDeployment": map[string]any{
					"id": "dpl_canary", "url": "web-canary.vercel.app", "readyState": "READY", "target": "production",
				},
				"currentCanaryPercentage": 25,
			},
		},
		ProjectCrons: map[string]any{
			"crons": map[string]any{
				"enabledAt":    int64(1716200000000),
				"disabledAt":   nil,
				"updatedAt":    int64(1716206800000),
				"deploymentId": "dpl_ready",
				"definitions": []any{
					map[string]any{"host": "web-ready.vercel.app", "path": "/api/cron/sync", "schedule": "0 5 * * *"},
					map[string]any{"host": "web-ready.vercel.app", "path": "/api/cron/digest", "schedule": "*/15 * * * *"},
				},
			},
		},
		CustomEnvironments: []map[string]any{
			{
				"id": "env_staging", "slug": "staging", "type": "preview",
				"description":   "Staging environment",
				"branchMatcher": map[string]any{"type": "startsWith", "pattern": "release/"},
				"domains":       []any{map[string]any{"name": "staging.example.com"}},
				"createdAt":     int64(1716100000000), "updatedAt": int64(1716206800000),
			},
			{
				"id": "env_qa", "slug": "qa", "type": "preview",
				"branchMatcher": map[string]any{"type": "equals", "pattern": "qa"},
				"createdAt":     int64(1716050000000), "updatedAt": int64(1716060000000),
			},
		},
		DeploymentChecks: []map[string]any{
			{"id": "check_lint", "name": "Lint", "status": "completed", "conclusion": "succeeded", "blocking": true, "integrationId": "icfg_lint", "startedAt": int64(1716206500000), "completedAt": int64(1716206520000)},
			{"id": "check_e2e", "name": "E2E", "status": "completed", "conclusion": "failed", "blocking": true, "integrationId": "icfg_e2e", "detailsUrl": "https://ci.example.com/runs/1", "rerequestable": true, "startedAt": int64(1716206500000), "completedAt": int64(1716206590000)},
			{"id": "check_perf", "name": "Lighthouse", "status": "running", "conclusion": "", "blocking": false, "integrationId": "icfg_perf", "startedAt": int64(1716206500000)},
		},
		BuildEvents: []map[string]any{
			{"type": "stdout", "created": int64(1716206500000), "payload": map[string]any{"text": "Running \"next build\""}},
			{"type": "stderr", "created": int64(1716206501000), "payload": map[string]any{"text": "Error: build failed", "statusCode": 500}},
			{"type": "exit", "created": int64(1716206502000), "payload": map[string]any{"text": "Command exited with 1"}},
		},
		RuntimeLogs: []map[string]any{
			{"level": "info", "source": "serverless", "message": "GET /api/health 200", "timestampInMs": int64(1716206600000), "requestMethod": "GET", "requestPath": "/api/health", "responseStatusCode": 200},
			{"level": "error", "source": "serverless", "message": "GET /api/users 500 boom", "timestampInMs": int64(1716206601000), "requestMethod": "GET", "requestPath": "/api/users", "responseStatusCode": 500},
		},
		Env: []map[string]any{
			{"id": "env_shared", "key": "KEY_SHARED", "target": []any{"production", "preview"}, "type": "encrypted", "value": "shared-val"},
			{"id": "env_apiprod", "key": "API_URL", "target": []any{"production"}, "type": "plain", "value": "https://prod.example.com"},
			{"id": "env_apiprev", "key": "API_URL", "target": []any{"preview"}, "type": "plain", "value": "https://preview.example.com"},
			{"id": "env_onlyprod", "key": "ONLY_PROD", "target": []any{"production"}, "type": "encrypted", "value": "p"},
			{"id": "env_onlyprev", "key": "ONLY_PREVIEW", "target": []any{"preview"}, "type": "encrypted", "value": "v"},
		},
		SharedEnv: []map[string]any{
			{"id": "env_shared_db", "key": "DATABASE_URL", "type": "encrypted", "target": []any{"production", "preview"}, "projectId": []any{"prj_web", "prj_api"}, "value": "postgres://shared", "createdAt": int64(1716000000000), "updatedAt": int64(1716206800000)},
			{"id": "env_shared_flag", "key": "FEATURE_X", "type": "plain", "target": []any{"production"}, "projectId": []any{"prj_web"}, "value": "on", "createdAt": int64(1716010000000), "updatedAt": int64(1716010000000)},
		},
		Domains: []map[string]any{
			{
				"name": "example.com", "verified": true, "serviceType": "external",
				"nameservers":         []any{"ns1.registrar.com", "ns2.registrar.com"},
				"intendedNameservers": []any{"ns1.vercel-dns.com", "ns2.vercel-dns.com"},
				"expiresAt":           int64(1763200000000), "renew": true,
			},
		},
		DomainConfig: map[string]any{
			"misconfigured":      true,
			"configuredBy":       "CNAME",
			"acceptedChallenges": []any{"dns-01"},
			"recommendedCNAME":   []any{map[string]any{"rank": 0, "value": "cname.vercel-dns.com"}},
		},
		DomainRecords: []map[string]any{
			{"id": "rec_1", "type": "A", "name": "", "value": "76.76.21.21", "ttl": 60},
			{"id": "rec_2", "type": "CNAME", "name": "www", "value": "cname.vercel-dns.com", "ttl": 60},
		},
		Certs: map[string]map[string]any{
			// cert_1 expires in the past (an already-expired cert, for --expiring triage)…
			"cert_1": {"id": "cert_1", "createdAt": int64(1716000000000), "expiresAt": int64(1763200000000), "autoRenew": true, "cns": []any{"example.com", "www.example.com"}},
			// …cert_2 expires far in the future.
			"cert_2": {"id": "cert_2", "createdAt": int64(1716000000000), "expiresAt": int64(2000000000000), "autoRenew": true, "cns": []any{"app.example.com"}},
		},
		Aliases: []map[string]any{
			{"uid": "alias_1", "alias": "example.com", "created": "2026-05-01T10:00:00.000Z"},
			{"uid": "alias_2", "alias": "web-ready.vercel.app", "created": "2026-05-01T10:00:00.000Z", "protectionBypass": map[string]any{"scope": "shareable-link"}},
		},
		Charges: []map[string]any{
			{"ServiceName": "Functions", "ChargeCategory": "Usage", "BilledCost": 12.50, "BillingCurrency": "USD", "ConsumedQuantity": 1000000.0, "ConsumedUnit": "invocations", "RegionId": "iad1", "ChargePeriodStart": "2026-06-01T00:00:00Z", "ChargePeriodEnd": "2026-06-02T00:00:00Z", "Tags": map[string]any{"ProjectName": "web", "ProjectId": "prj_web"}},
			{"ServiceName": "Bandwidth", "ChargeCategory": "Usage", "BilledCost": 40.00, "BillingCurrency": "USD", "ConsumedQuantity": 200.0, "ConsumedUnit": "GB", "RegionId": "iad1", "ChargePeriodStart": "2026-06-01T00:00:00Z", "ChargePeriodEnd": "2026-06-02T00:00:00Z", "Tags": map[string]any{"ProjectName": "web", "ProjectId": "prj_web"}},
			{"ServiceName": "Functions", "ChargeCategory": "Usage", "BilledCost": 3.00, "BillingCurrency": "USD", "ConsumedQuantity": 50000.0, "ConsumedUnit": "invocations", "RegionId": "sfo1", "ChargePeriodStart": "2026-06-01T00:00:00Z", "ChargePeriodEnd": "2026-06-02T00:00:00Z", "Tags": map[string]any{"ProjectName": "api", "ProjectId": "prj_api"}},
		},
		Webhooks: []map[string]any{
			{"id": "hook_deploys", "url": "https://hooks.example.com/vercel", "events": []any{"deployment.created", "deployment.succeeded", "deployment.error"}, "projectIds": []any{"prj_web"}, "createdAt": int64(1716000000000), "updatedAt": int64(1716206800000)},
			{"id": "hook_all", "url": "https://hooks.example.com/audit", "events": []any{"deployment.error"}, "createdAt": int64(1716010000000), "updatedAt": int64(1716010000000)},
		},
		EdgeConfigs: []map[string]any{
			{"id": "ecfg_flags", "slug": "flags", "itemCount": 2, "sizeInBytes": 128, "createdAt": int64(1716000000000), "updatedAt": int64(1716206800000)},
			{"id": "ecfg_redirects", "slug": "redirects", "itemCount": 0, "createdAt": int64(1716010000000), "updatedAt": int64(1716010000000)},
		},
		EdgeConfigItems: map[string][]map[string]any{
			"ecfg_flags": {
				{"key": "maintenance_mode", "value": false},
				{"key": "new_checkout", "value": map[string]any{"enabled": true, "rollout": 25}},
			},
		},
		FirewallConfig: map[string]any{
			"firewallEnabled": true,
			"version":         7,
			"rules": []any{
				map[string]any{"id": "rule_bots", "name": "block-bad-bots", "active": true, "action": map[string]any{"mitigate": map[string]any{"action": "deny"}}},
				map[string]any{"id": "rule_old", "name": "legacy-rule", "active": false},
			},
			"ips": []any{
				map[string]any{"id": "ip_1", "hostname": "web.example.com", "ip": "203.0.113.7", "action": "deny"},
			},
			"managedRules": map[string]any{
				"owasp":          map[string]any{"active": true},
				"bot_protection": map[string]any{"active": false},
			},
		},
		AttackStatus: map[string]any{
			"anomalies": []any{
				map[string]any{"projectId": "prj_web", "atMinute": int64(1716206400000), "affectedHostMap": map[string]any{"web.example.com": map[string]any{"observedMax": 12000}}},
			},
		},
		FirewallBypass: map[string]any{
			"result": []any{
				map[string]any{"domain": "web.example.com", "sourceIp": "198.51.100.9", "allSources": false},
			},
		},
		Drains: []map[string]any{
			{
				"id": "drain_logs", "name": "datadog-logs", "status": "enabled",
				"schemas":    map[string]any{"log": map[string]any{"version": 1}},
				"delivery":   map[string]any{"url": "https://http-intake.datadoghq.com/v1/input?token=SECRET"},
				"projectIds": []any{"prj_web"},
				"createdAt":  int64(1716000000000),
			},
			{
				"id": "drain_traces", "name": "honeycomb-traces", "status": "errored", "disabled": true,
				"schemas":   map[string]any{"trace": map[string]any{"version": 1}},
				"delivery":  map[string]any{"url": "https://api.honeycomb.io/v1/traces"},
				"createdAt": int64(1716010000000),
			},
		},
	}
}
