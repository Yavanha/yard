package runtime

import "yard/internal/provider/lima"

type LocalVM struct {
	client lima.Client
	vmName string
}

func NewLocalVM(client lima.Client, vmName string) LocalVM {
	return LocalVM{
		client: client,
		vmName: vmName,
	}
}

func (target LocalVM) Exec(command []string) error {
	return target.client.Exec(target.vmName, command)
}

func (target LocalVM) ExecOutput(command []string) ([]byte, error) {
	return target.client.ExecOutput(target.vmName, command)
}
