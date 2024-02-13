module market/producer

go 1.21.5

replace market/service => ../service

require market/service v0.0.0-00010101000000-000000000000

require github.com/google/uuid v1.6.0 // indirect
