
SYMBOL = -s
LIMIT = -l

rest:
	go run main.go rest --l $(LIMIT) --s $(SYMBOL)

ws:
	go run main.go ws --l $(LIMIT) --s $(SYMBOL)