package main

func (s *server) routes() {
	s.router.HandleFunc("/api/v1/best/{kernelrelease}", s.bestKmod()).Methods("POST")
	s.router.HandleFunc("/api/v1/hello", s.hello()).Methods("GET")
}
