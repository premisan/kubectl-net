package cmd

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestParseDBClientArgs(t *testing.T) {
	defaults := commonFlags{
		debugImage:    "nicolaka/netshoot:latest",
		imageExplicit: false,
	}

	tests := []struct {
		name         string
		defaultImage string
		rawArgs      []string
		defaults     commonFlags
		wantPod      string
		wantNS       string
		wantImage    string
		wantArgs     []string
		wantHelp     bool
		expectError  bool
		errContains  string
	}{
		{
			name:         "mysql default image and full flag passthrough including -h -u -p",
			defaultImage: defaultMySQLImage,
			rawArgs:      []string{"-n", "prod", "my-pod", "-h", "mysql-svc", "-P", "3306", "-u", "admin", "-p", "mydb"},
			defaults:     defaults,
			wantPod:      "my-pod",
			wantNS:       "prod",
			wantImage:    "mysql:latest",
			wantArgs:     []string{"-h", "mysql-svc", "-P", "3306", "-u", "admin", "-p", "mydb"},
		},
		{
			name:         "psql default image and -c/-h/-v passthrough",
			defaultImage: defaultPsqlImage,
			rawArgs:      []string{"-n", "prod", "-c", "app", "pg-client", "-h", "pg.internal", "-U", "postgres", "-d", "mydb", "-c", "SELECT 1;", "-v", "ON_ERROR_STOP=1"},
			defaults:     defaults,
			wantPod:      "pg-client",
			wantNS:       "prod",
			wantImage:    "postgres:latest",
			wantArgs:     []string{"-h", "pg.internal", "-U", "postgres", "-d", "mydb", "-c", "SELECT 1;", "-v", "ON_ERROR_STOP=1"},
		},
		{
			name:         "redis default image and -n/-c/-h passthrough after POD_NAME",
			defaultImage: defaultRedisImage,
			rawArgs:      []string{"-n", "cache", "app-pod", "-h", "redis-svc", "-p", "6379", "--tls", "-c", "-n", "2", "PING"},
			defaults:     defaults,
			wantPod:      "app-pod",
			wantNS:       "cache",
			wantImage:    "redis:latest",
			wantArgs:     []string{"-h", "redis-svc", "-p", "6379", "--tls", "-c", "-n", "2", "PING"},
		},
		{
			name:         "explicit --image before POD_NAME overrides subcommand default",
			defaultImage: defaultMySQLImage,
			rawArgs:      []string{"--image", "registry.internal/mariadb:latest", "my-pod", "-h", "db", "-u", "root", "-p"},
			defaults:     defaults,
			wantPod:      "my-pod",
			wantImage:    "registry.internal/mariadb:latest",
			wantArgs:     []string{"-h", "db", "-u", "root", "-p"},
		},
		{
			name:         "explicit root-level --image overrides subcommand default",
			defaultImage: defaultPsqlImage,
			rawArgs:      []string{"my-pod", "-h", "pg"},
			defaults: commonFlags{
				debugImage:    "postgres:alpine",
				imageExplicit: true,
			},
			wantPod:   "my-pod",
			wantImage: "postgres:alpine",
			wantArgs:  []string{"-h", "pg"},
		},
		{
			name:         "no client args after POD_NAME is allowed (for localhost/sidecar DB)",
			defaultImage: defaultRedisImage,
			rawArgs:      []string{"redis-pod"},
			defaults:     defaults,
			wantPod:      "redis-pod",
			wantImage:    "redis:latest",
			wantArgs:     nil,
		},
		{
			name:         "-h before POD_NAME shows help",
			defaultImage: defaultMySQLImage,
			rawArgs:      []string{"-h"},
			defaults:     defaults,
			wantHelp:     true,
		},
		{
			name:         "no args shows help",
			defaultImage: defaultMySQLImage,
			rawArgs:      []string{},
			defaults:     defaults,
			wantHelp:     true,
		},
		{
			name:         "DB option placed before POD_NAME is rejected",
			defaultImage: defaultMySQLImage,
			rawArgs:      []string{"-u", "root", "my-pod", "-h", "db"},
			defaults:     defaults,
			expectError:  true,
			errContains:  "before POD_NAME",
		},
		{
			name:         "missing POD_NAME after global flags returns error",
			defaultImage: defaultRedisImage,
			rawArgs:      []string{"-n", "prod"},
			defaults:     defaults,
			expectError:  true,
			errContains:  "target pod name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, showHelp, err := parseDBClientArgs(tt.rawArgs, tt.defaults, tt.defaultImage, "usage")
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if showHelp != tt.wantHelp {
				t.Fatalf("showHelp = %v, want %v", showHelp, tt.wantHelp)
			}
			if tt.wantHelp {
				return
			}
			if got.podName != tt.wantPod {
				t.Fatalf("podName = %q, want %q", got.podName, tt.wantPod)
			}
			if got.flags.namespace != tt.wantNS {
				t.Fatalf("namespace = %q, want %q", got.flags.namespace, tt.wantNS)
			}
			if got.debugImage != tt.wantImage {
				t.Fatalf("debugImage = %q, want %q", got.debugImage, tt.wantImage)
			}
			if !reflect.DeepEqual(got.clientArgs, tt.wantArgs) {
				t.Fatalf("clientArgs = %#v, want %#v", got.clientArgs, tt.wantArgs)
			}
		})
	}
}

func TestRedactSensitiveArgs(t *testing.T) {
	tests := []struct {
		name   string
		binary string
		args   []string
		want   []string
	}{
		{
			name:   "mysql interactive -p stays untouched",
			binary: "mysql",
			args:   []string{"-h", "db", "-u", "root", "-p", "mydb"},
			want:   []string{"-h", "db", "-u", "root", "-p", "mydb"},
		},
		{
			name:   "mysql inline -psecret and --password=secret are masked",
			binary: "mysql",
			args:   []string{"-h", "db", "-u", "root", "-pSuperSecret", "--password=AnotherSecret"},
			want:   []string{"-h", "db", "-u", "root", "-p***", "--password=***"},
		},
		{
			name:   "redis -a and --pass credentials are masked",
			binary: "redis-cli",
			args:   []string{"-h", "redis", "-a", "s3cr3t", "--pass", "p2", "--pass=p3", "-aInline", "PING"},
			want:   []string{"-h", "redis", "-a", "***", "--pass", "***", "--pass=***", "-a***", "PING"},
		},
		{
			name:   "psql connection URI and key-value password are masked",
			binary: "psql",
			args:   []string{"postgresql://admin:topsecret@pg.internal:5432/mydb?sslmode=require", "host=pg.internal user=admin password=mysecret dbname=mydb"},
			want:   []string{"postgresql://admin:***@pg.internal:5432/mydb?sslmode=require", "host=pg.internal user=admin password=*** dbname=mydb"},
		},
		{
			name:   "redis URI with password is masked",
			binary: "redis-cli",
			args:   []string{"-u", "rediss://default:mypassword@redis.internal:6379/0"},
			want:   []string{"-u", "rediss://default:***@redis.internal:6379/0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactSensitiveArgs(tt.binary, tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("redactSensitiveArgs(%q, %#v) = %#v, want %#v", tt.binary, tt.args, got, tt.want)
			}
		})
	}
}

func TestDBSubcommandsRegisteredAndHelp(t *testing.T) {
	for _, sub := range []string{"mysql", "psql", "redis"} {
		t.Run(sub+" help", func(t *testing.T) {
			buf := new(bytes.Buffer)
			RootCmd.SetOut(buf)
			RootCmd.SetErr(buf)
			RootCmd.SetArgs([]string{sub, "--help"})

			if err := RootCmd.Execute(); err != nil {
				t.Fatalf("unexpected error executing %s --help: %v", sub, err)
			}
			if !strings.Contains(buf.String(), sub) {
				t.Fatalf("help output for %s did not contain subcommand name: %s", sub, buf.String())
			}
		})
	}
}
