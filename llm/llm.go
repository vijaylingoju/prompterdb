// llm/llm.go
package llm

type LLM interface {
	GenerateSQL(prompt string, schema string) (string, error)
	Name() string
}
