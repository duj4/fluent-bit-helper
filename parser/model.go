package parser

// ParseInput represents user parse input
type ParseInput struct {
	// parser
	Format     string
	TimeKey    string
	TimeValue  string
	Timezone   string
	ParserName string

	// filter
	Match   string
	KeyName string
}

// Parser represents a signle Fluent-Bit parser configuration
type Parser struct {
	Name       string `yaml:"name"`
	Format     string `yaml:"format"`
	TimeKey    string `yaml:"time_key"`
	TimeFormat string `yaml:"time_format"`
	TimeKeep   string `yaml:"time_keep"`
	TimeOffset string `yaml:"time_offset,omitempty"`
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
