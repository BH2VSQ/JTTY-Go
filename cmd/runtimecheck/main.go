package main

import (
    "flag"
    "fmt"
    "os"
    "github.com/BH2VSQ/jtty-go/internal/radio"
)

func main() {
    dir:=flag.String("dir","bin","Hamlib runtime directory")
    flag.Parse()
    if err:=radio.VerifyRuntime(*dir);err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}
    fmt.Println("Hamlib Windows amd64 runtime hashes verified")
}
