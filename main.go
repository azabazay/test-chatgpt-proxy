package main

func main() {
	redisCli := NewRedisCli()
	defer redisCli.Close()

	server := NewAPIServer(
		":8000",
		redisCli,
	)

	server.Run()
}
