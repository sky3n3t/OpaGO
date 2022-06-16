package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

var vm VM

func main() {
	wasm, err := os.ReadFile("./policy.wasm")
	if err != nil {
		log.Panic(err)
	}
	data, err := os.ReadFile("./data.json")
	ds := ""
	for _, ch := range data {
		ds += string(ch)
	}
	ds2 := ""
	for _, ch := range ds {
		if string(ch) != "\n" && string(ch) != " " && string(ch) != "\t" {
			ds2 += string(ch)
		}
	}
	if err != nil {
		log.Panic(err)
	}
	opts := vmOpts{wasm, []byte(ds2), 15}
	vm = newVM(opts)
	fmt.Println(vm.Eval([]byte("{\"user\":\"alice\",\"method\":\"get\"}")))
	router := gin.Default()
	router.GET("/data", getData)
	router.POST("/data", postData)
	router.GET("/policy", getPolicy)
	router.POST("/policy", postPolicy)
	router.POST("/eval", eval)
	router.Run("localhost:8080")
}
func getData(c *gin.Context) {
	data := vm.getData()
	var jsVal any
	json.Unmarshal([]byte(data), &jsVal)
	c.IndentedJSON(http.StatusOK, jsVal)
}
func postData(c *gin.Context) {
	data := c.GetString("data")
	vm.SetData([]byte(data))
	vm.LoadData()
}
func getPolicy(c *gin.Context) {
	data := vm.getPolicy()
	c.IndentedJSON(http.StatusOK, data)
}
func postPolicy(c *gin.Context) {
	data := c.GetString("policy")
	vm.SetPolicy([]byte(data))
	vm.LoadPolicy()
}
func eval(c *gin.Context) {
	var jsVal any
	c.BindJSON(&jsVal)
	input, _ := json.Marshal(&jsVal)
	out := ""
	for _, ch := range input {
		out += string(ch)
	}
	fmt.Println(out)
	fmt.Println(vm.Eval([]byte(out)))
	json.Unmarshal([]byte(vm.Eval([]byte(out))), &jsVal)
	c.IndentedJSON(http.StatusOK, jsVal)
}
