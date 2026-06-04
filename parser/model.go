package parser

// ParseInput represents user parse input
type ParserInput struct {
	// parser
	Format     string
	TimeKey    string
	TimeValue  string
	Timezone   string
	ParserName string

	// regex parser
	Snippet     string
	RegexFields []RegexField

	// multiline parser
	EnableMultiline     bool
	MultilineParserName string

	// filter
	Match   string
	KeyName string
}

// RegexField represents one user-defined key/value pair.
type RegexField struct {
	Key      string
	Value    string
	Optional bool
	Pattern  string
}

// Parser represents a signle Fluent-Bit parser configuration
type Parser struct {
	Name       string `yaml:"name"`
	Format     string `yaml:"format"`
	Regex      string `yaml:"regex,omitempty"`
	TimeKey    string `yaml:"time_key"`
	TimeFormat string `yaml:"time_format"`
	TimeKeep   string `yaml:"time_keep"`
	TimeOffset string `yaml:"time_offset,omitempty"`
}

// MultilineParser represents Fluent-Bit multiline parser configuration
type MultilineParser struct {
	Name         string          `yaml:"name"`
	Type         string          `yaml:"type"`
	FlushTimeout int             `yaml:"flush_timeout"`
	Rules        []MultilineRule `yaml:"rules"`
}

// MultilineRule represents one multiline parser rule
type MultilineRule struct {
	State     string `yaml:"state"`
	Regex     string `yaml:"regex"`
	NextState string `yaml:"next_state"`
}

// Filter represents Fluent-Bit parser filter
type Filter struct {
	Name        string `yaml:"name"`
	Match       string `yaml:"match"`
	KeyName     string `yaml:"key_name"`
	Parser      string `yaml:"parser"`
	ReserveData string `yaml:"reserve_data"`
}

// Token represents parsed timestamp token
type Token struct {
	Type  string
	Value string
}
