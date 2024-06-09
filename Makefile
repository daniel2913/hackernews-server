EXEC=Hackernews

all : ${EXEC} 

${EXEC}: client/dist/index.html
	go build -o ${EXEC} .

client/dist/index.html:
	git submodule update --remote
	npm i --prefix client/
	npm  run build --prefix client/ 

clean:
	rm -f $(EXEC) 
	rm -rf client/dist
