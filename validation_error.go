package flam

type ValidationError struct {
	ParamId      int
	ParamName    string
	ErrorId      int
	ErrorMessage string
}
