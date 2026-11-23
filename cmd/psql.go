package cmd

const defaultPsqlImage = "postgres:latest"

var psqlCmd = newDBClientCommand(dbClientSpec{
	name:            "psql",
	binary:          "psql",
	defaultImage:    defaultPsqlImage,
	containerPrefix: "kubectl-net-psql",
	use:             "psql [GLOBAL_FLAGS...] [POD_NAME] [PSQL_ARGS...]",
	aliases:         []string{"postgres", "postgresql"},
	short:           "Connect to a PostgreSQL server from inside a target pod using psql",
	long: `Connect to a PostgreSQL server from inside a Kubernetes pod's network namespace
using an Ephemeral Debug Container (default image: postgres:latest, override via --image).

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. All arguments after POD_NAME are passed verbatim to psql.

Interactive sessions automatically allocate a TTY (safe password prompt and full REPL
support); piped or redirected I/O automatically switches to non-TTY mode.`,
	example: `  # Interactive PostgreSQL connection:
  kubectl net psql my-pod -h postgres-svc.default.svc -U postgres -d mydb

  # Target pod in a specific namespace and run a single query:
  kubectl net psql -n production my-pod -h pg.internal -p 5432 -U appuser -d appdb -c "SELECT version();"

  # Connect using a PostgreSQL connection URI:
  kubectl net psql -n production my-pod "postgresql://postgres@pg.internal:5432/mydb?sslmode=require"

  # Override debug container image (e.g. lightweight alpine or private registry):
  kubectl net psql --image postgres:alpine my-pod -h pg-svc -U postgres`,
})

func init() {
	RootCmd.AddCommand(psqlCmd)
}
