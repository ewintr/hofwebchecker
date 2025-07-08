
deploy:
	go build -o hofwebchecker .
	scp hofwebchecker server:
	ssh server sudo systemctl stop hofwebchecker.service
	ssh server sudo mv hofwebchecker /usr/local/bin/hofwebchecker
	ssh server sudo systemctl start hofwebchecker.service
	rm hofwebchecker
