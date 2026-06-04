package config

import "fluent-bit-helper/parser"

// UserInput represents user input
type UserInput struct {
	Mode string

	FluentBitLogFile string
	StoragePath      string

	LogPaths     []string
	DBPath       string
	ReadFromHead bool
	Tag          string

	Output string

	// parser
	EnableParser bool
	ParserInput  parser.ParserInput

	// loki
	LokiHost               string
	TenantID               string
	TLSCAFile              string
	TLSCrtFile             string
	TLSKeyFile             string
	LokiLineFormat         string
	LokiLabels             []LokiLabel
	LokiStructuredMetadata []LokiStructuredMetadata
}

// LokiLabel represents one Loki label mapping
type LokiLabel struct {
	Name  string
	Field string
}

// LokiStructuredMetadata represents one Loki structured metadata mapping
type LokiStructuredMetadata struct {
	Name  string
	Field string
}

// FluentBitConfig represents fluent-bit default config
type FluentBitConfig struct {
	Env              Env                      `yaml:"env"`
	Service          Service                  `yaml:"service"`
	Parsers          []parser.Parser          `yaml:"parsers,omitempty"`
	MultilineParsers []parser.MultilineParser `yaml:"multiline_parsers,omitempty"`
	Pipeline         Pipeline                 `yaml:"pipeline"`
}

// Env represents environment variables
type Env struct {
	FLBLogFile  string `yaml:"FLB_LOG_FILE,omitempty"`
	StoragePath string `yaml:"STORAGE_PATH,omitempty"`

	LogPath string `yaml:"LOG_PATH"`
	DBPath  string `yaml:"DB_PATH,omitempty"`
	Tag     string `yaml:"TAG"`

	LokiHost   string `yaml:"LOKI_HOST,omitempty"`
	TenantID   string `yaml:"TENANT_ID,omitempty"`
	TLSCAFile  string `yaml:"TLS_CA_FILE,omitempty"`
	TLSCrtFile string `yaml:"TLS_CRT_FILE,omitempty"`
	TLSKeyFile string `yaml:"TLS_KEY_FILE,omitempty"`
}

// Service represents service configuration
type Service struct {
	LogFile  string `yaml:"log_file,omitempty"`
	LogLevel string `yaml:"log_level"`

	SchedulerBase int `yaml:"scheduler.base,omitempty"`
	SchedulerCap  int `yaml:"scheduler.cap,omitempty"`

	StoragePath                      string `yaml:"storage.path,omitempty"`
	StorageSync                      string `yaml:"storage.sync,omitempty"`
	StorageDeleteIrrecoverableChunks string `yaml:"storage.delete_irrecoverable_chunks,omitempty"`
}

// Pipeline represents pipeline configuration
type Pipeline struct {
	Inputs  []interface{} `yaml:"inputs"`
	Filters []interface{} `yaml:"filters,omitempty"`
	Outputs []interface{} `yaml:"outputs"`
}

// TailInput represents tail input plugin configuration.
type TailInput struct {
	Name string `yaml:"name"`

	Tag          string `yaml:"tag"`
	Path         string `yaml:"path"`
	PathKey      string `yaml:"path_key"`
	DB           string `yaml:"db,omitempty"`
	ReadFromHead string `yaml:"read_from_head,omitempty"`

	MultilineParser string `yaml:"multiline.parser,omitempty"`

	SkipEmptyLines  string `yaml:"skip_empty_lines"`
	RefreshInterval int    `yaml:"refresh_interval"`
	StorageType     string `yaml:"storage.type,omitempty"`
	Threaded        string `yaml:"threaded"`
}

// StdoutOutput represents stdout output plugin configuration.
type StdoutOutput struct {
	Name  string `yaml:"name"`
	Match string `yaml:"match"`
}

// LokiOutput represents loki output plugin configuration.
type LokiOutput struct {
	Name  string `yaml:"name"`
	Match string `yaml:"match"`

	Host               string `yaml:"host,omitempty"`
	Port               int    `yaml:"port,omitempty"`
	TenantID           string `yaml:"tenant_id,omitempty"`
	Labels             string `yaml:"labels,omitempty"`
	StructuredMetadata string `yaml:"structured_metadata,omitempty"`

	LineFormat string `yaml:"line_format,omitempty"`

	TLS        string `yaml:"tls,omitempty"`
	TLSCAFile  string `yaml:"tls.ca_file,omitempty"`
	TLSCrtFile string `yaml:"tls.crt_file,omitempty"`
	TLSKeyFile string `yaml:"tls.key_file,omitempty"`

	RetryLimit string `yaml:"retry_limit,omitempty"`

	StorageTotalLimitSize string `yaml:"storage.total_limit_size,omitempty"`

	NetConnectTimeout       int    `yaml:"net.connect_timeout,omitempty"`
	NetKeepaliveIdleTimeout int    `yaml:"net.keepalive_idle_timeout,omitempty"`
	NetDNSResolver          string `yaml:"net.dns.resolver,omitempty"`

	Workers int `yaml:"workers,omitempty"`
}
