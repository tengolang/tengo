package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/tengolang/tengo/v3"
)

const enemyScript = `
health := 100
rage := 0
guarding := false

_ready := func() {
	health = 100
	rage = 0
	guarding = false
}

_on_turn := func(player_action) {
	damage := 0
	message := ""

	if player_action == "attack" {
		damage = 18
		message = "You slash the enemy."
	} else if player_action == "heavy" {
		damage = 32
		rage += 1
		message = "You commit to a heavy strike."
	} else if player_action == "defend" {
		damage = 0
		message = "You brace yourself."
	} else if player_action == "wait" {
		damage = 0
		message = "You wait."
	} else {
		damage = 0
		message = "Unknown action."
	}

	if guarding {
		damage = damage / 2
		guarding = false
	}

	if player_action == "defend" {
		damage = damage / 2
	}

	health -= damage

	if health <= 0 {
		return {
			"enemy_action": "dead",
			"enemy_damage": 0,
			"message": message + " The enemy collapses.",
		}
	}

	rage += 1

	if rage >= 4 {
		rage = 0
		return {
			"enemy_action": "heavy_attack",
			"enemy_damage": 24,
			"message": message + " The enemy unleashes a heavy counterattack!",
		}
	}

	if health < 35 && !guarding {
		guarding = true
		return {
			"enemy_action": "guard",
			"enemy_damage": 0,
			"message": message + " The enemy raises its guard.",
		}
	}

	return {
		"enemy_action": "attack",
		"enemy_damage": 12,
		"message": message + " The enemy strikes back.",
	}
}

_get_state := func() {
	return {
		"health": health,
		"rage": rage,
		"guarding": guarding,
	}
}
`

func main() {
	script := tengo.NewScript([]byte(enemyScript))

	compiled, err := script.Compile()
	if err != nil {
		log.Fatal(err)
	}

	if err := compiled.Run(); err != nil {
		log.Fatal(err)
	}

	playerHealth := 100
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Tengo Duel")
	fmt.Println("Commands: attack, heavy, defend, quit")
	fmt.Println()

	for turn := 1; ; turn++ {
		state, err := compiled.Call("_get_state")
		if err != nil {
			log.Fatal(err)
		}

		stateMap := state.Map()
		enemyHealth := asInt(stateMap["health"])

		fmt.Printf("Turn %d\n", turn)
		fmt.Printf("You: %d HP | Enemy: %d HP\n", playerHealth, enemyHealth)
		fmt.Print("> ")

		line, _ := reader.ReadString('\n')
		action := strings.ToLower(strings.TrimSpace(line))

		if action == "quit" {
			fmt.Println("You flee.")
			return
		}

		if action != "attack" && action != "heavy" && action != "defend" {
			action = "wait"
		}

		result, err := compiled.Call("_on_turn", action)
		if err != nil {
			log.Fatal(err)
		}

		resultMap := result.Map()

		message := asString(resultMap["message"])
		enemyAction := asString(resultMap["enemy_action"])
		enemyDamage := asInt(resultMap["enemy_damage"])

		fmt.Println(message)

		playerHealth -= enemyDamage

		if enemyAction == "dead" {
			fmt.Println("You win.")
			return
		}

		if enemyDamage > 0 {
			fmt.Printf("You take %d damage.\n", enemyDamage)
		}

		if playerHealth <= 0 {
			fmt.Println("You lose.")
			return
		}

		fmt.Println()
	}
}

func asInt(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
