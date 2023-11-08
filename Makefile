
#DSN = 'postgresql://zimmer:6969@localhost:/test?sslmode=disable'
DSN = 'postgresql://praktikum:praktikum@localhost:5432/praktikum?sslmode=disable'

defaultDBConn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		`localhost`, `5432`, `praktikum`, `praktikum`, `praktikum`)

build: build-agent build-server

build-server:
	go build -o cmd/server/server cmd/server/*.go

build-agent:
	go build -o cmd/agent/agent cmd/agent/*.go


run-server: build-server
	./cmd/server/server -a="localhost:8080" -i=0 -d=$(DSN) -k=testkey

run-agent: build-agent
	./cmd/agent/agent -a="localhost:8080" -r=10 -p=2 -k=testkey -l=2

stattest:
	go vet -vettool=statictest ./...

autotests: build autotests14

autotests1:
	metricstest -test.v -test.run=^TestIteration1$$ -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server
autotests2: autotests1
	metricstest -test.v -test.run=^TestIteration2[AB]*$$ -source-path=. -agent-binary-path=cmd/agent/agent
autotests3: autotests2
	metricstest -test.v -test.run=^TestIteration3[AB]*$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server
autotests4: autotests3
	metricstest -test.v -test.run=^TestIteration4$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080"
autotests5: autotests4
	metricstest -test.v -test.run=^TestIteration5$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080"
autotests6: autotests5
	metricstest -test.v -test.run=^TestIteration6$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080"
autotests7: autotests6
	metricstest -test.v -test.run=^TestIteration7$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080"
autotests8: autotests7
	metricstest -test.v -test.run=^TestIteration8$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080"
autotests9: autotests8
	metricstest -test.v -test.run=^TestIteration9$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080" -file-storage-path=/tmp/metrics-db.json
autotests10: autotests9
	metricstest -test.v -test.run=^TestIteration10[AB]$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080" -database-dsn=$(DSN)
autotests11: autotests10
	metricstest -test.v -test.run=^TestIteration11$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080" -database-dsn=$(DSN)
autotests12: autotests11
	metricstest -test.v -test.run=^TestIteration12$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080" -database-dsn=$(DSN)
autotests13: autotests12
	metricstest -test.v -test.run=^TestIteration13$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080" -database-dsn=$(DSN)
autotests14: autotests13
	metricstest -test.v -test.run=^TestIteration14$$ -source-path=. -agent-binary-path=cmd/agent/agent -binary-path=cmd/server/server -server-port="8080" -database-dsn=$(DSN) -key="testkey"
