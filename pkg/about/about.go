package about

var (
	// Version is the current version of the app, generated at build time
	Version   = "dev"
	Epoch     = "-1"
	Timestamp = "dev"
	Build     = "dev"
)

type Ab struct {
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
	Epoch     string `json:"epoch"`
	Build     string `json:"build"`
}

var About Ab

func init() {
	About = Ab{
		Epoch:     Epoch,
		Timestamp: Timestamp,
		Version:   Version,
		Build:     Build,
	}
}
