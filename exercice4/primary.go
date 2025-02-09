package main

import (
	"fmt"
	"time"
)

func main() {

	// Print a message
	fmt.Println("The program will terminate in 5 seconds...")

	// Sleep for 5 seconds
	time.Sleep(2 * time.Second)

	// Program terminates automatically after the sleep
	fmt.Println("Program terminated.")

	// fmt.Print("-- Primary Phase -- \n")

	// var count int = 1

	// file, err := os.Create("testFile.txt")  //create a new file

	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	// ticker := time.NewTicker(700 * time.Millisecond)
	// stop := make(chan bool)

	// exec.Command("cmd", "/C", "start", "powershell", "go", "run", "backup.go").Run()

	// go func() {
	//     for{
	//         select {
	//         case <-ticker.C:
	//             d1 := []byte(strconv.Itoa(count) + "\n")
	// 			file.Write(d1)

	// 			fmt.Print(count, "\n")

	// 			count++
	// 			if(count == 5){
	// 				exec.Command("cmd", "/C", "start", "powershell", "go", "run", "backup.go").Run()
	// 			}
	//         }
	//     }
	// }()

	// stop <- true

}
