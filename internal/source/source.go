package source

type Source interface {
	Load() (interface{}, error)
}
