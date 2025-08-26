// Originally from go-gibberish
// Copyright (c) 2015 Rob Renaud
// Licensed under the MIT License. See LICENSES/MIT.txt.
//
// Modifications copyright (c) 2025 Grafana Labs
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/grafana/clusterurl/pkg/consts"
	"github.com/grafana/clusterurl/pkg/gibberish"
	"github.com/grafana/clusterurl/pkg/persistence"
	"github.com/grafana/clusterurl/pkg/training"
)

var (
	performTraining bool
)

func main() {

	flag.BoolVar(&performTraining, "train", false, "train")
	flag.Parse()

	if performTraining {
		err := training.TrainModel(consts.AcceptedCharacters, "assets/big.txt", "assets/good.txt", "assets/bad.txt", "pkg/clusterurl/model.json")
		if err != nil {
			log.Fatal(err)
		}

		return
	}

	reader := bufio.NewReader(os.Stdin)
	data, err := persistence.LoadKnowledgeBase("pkg/clusterurl/model.json")
	if err != nil {
		log.Fatal(err)
	}

	for {

		fmt.Print("Insert something to check: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		isGibberish := gibberish.IsGibberish(input, data)
		fmt.Println(fmt.Sprintf("Input: %s: is gibberish? %v\n", input, isGibberish))

	}

}
