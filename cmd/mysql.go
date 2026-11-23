package cmd

const defaultMySQLImage = "mysql:latest"

var mysqlCmd = newDBClientCommand(dbClientSpec{
	name:            "mysql",
	binary:          "mysql",
	defaultImage:    defaultMySQLImage,
	containerPrefix: "kubectl-net-mysql",
	use:             "mysql [GLOBAL_FLAGS...] [POD_NAME] [MYSQL_ARGS...]",
	short:           "Connect to a MySQL server from inside a target pod using the mysql client",
	long: `Connect to a MySQL/MariaDB server from inside a Kubernetes pod's network namespace
using an Ephemeral Debug Container (default image: mysql:latest, override via --image).

All kubectl-net flags (-n, -c, --image, --context, --kubeconfig, -v) must be placed
BEFORE POD_NAME. All arguments after POD_NAME are passed verbatim to the mysql CLI.

Interactive sessions automatically allocate a TTY (safe password prompt with -p and
full REPL support); piped or redirected I/O automatically switches to non-TTY mode.`,
	example: `  # Interactive MySQL connection with password prompt:
  kubectl net mysql my-pod -h mysql-svc.default.svc -u admin -p mydb

  # Target pod in a specific namespace and custom port:
  kubectl net mysql -n production my-pod -h 10.0.10.25 -P 3306 -u appuser -p mydb

  # Run a single test query:
  kubectl net mysql -n production my-pod -h mysql-svc -u admin -p -e "SELECT 1;"

  # Override debug container image (e.g. private registry or MariaDB):
  kubectl net mysql --image mariadb:latest my-pod -h db.internal -u root -p`,
})

func init() {
	RootCmd.AddCommand(mysqlCmd)
}
