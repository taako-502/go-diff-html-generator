.PHONY: diff diff-json test vet check clean

diff:
	go run . -before examples/before.txt -after examples/after.txt -out diff.html

diff-json:
	go run . -before examples/before.json -after examples/after.json -out diff-json.html -mode json

test:
	go test ./...

vet:
	go vet ./...

check: test vet

clean:
	rm -f diff.html diff-json.html
