package runtime

type Target interface {
	Exec(command []string) error
	ExecOutput(command []string) ([]byte, error)
}
