package notify

type Spec struct {
	Notify *Notify `json:"notify,omitempty" yaml:"notify,omitempty"`
}

type Notify struct {
	Slack []*Slack `json:"slack,omitempty" yaml:"slack,omitempty"`
}

type Slack struct {
	Url   string `json:"url,omitempty" yaml:"url,omitempty"`
	Token string `json:"token,omitempty" yaml:"token,omitempty"`
	Data  string `json:"data,omitempty" yaml:"data,omitempty"`
}
