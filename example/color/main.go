package main

import "fmt"

func main() {
	// ANSI color codes
	reset := "\033[0m"

	// Line 1: Basic foreground colors
	fmt.Print("\033[31mRed text" + reset)
	fmt.Print(" \033[32mGreen text" + reset)
	fmt.Print(" \033[33mYellow text" + reset)
	fmt.Print(" \033[34mBlue text" + reset)
	fmt.Println(" \033[35mMagenta text" + reset)

	// Line 2: Background colors
	fmt.Print("\033[41;37mRed BG" + reset)
	fmt.Print(" \033[42;30mGreen BG" + reset)
	fmt.Print(" \033[43;30mYellow BG" + reset)
	fmt.Print(" \033[44;37mBlue BG" + reset)
	fmt.Println(" \033[45;37mMagenta BG" + reset)

	// Line 3: Mixed combinations
	fmt.Print("\033[31;47mRed on White" + reset)
	fmt.Print(" \033[33;44mYellow on Blue" + reset)
	fmt.Print(" \033[36;40mCyan on Black" + reset)
	fmt.Println(" \033[32;45mGreen on Magenta" + reset)

	// Line 4: Bright/bold colors
	fmt.Print("\033[1;31mBright Red" + reset)
	fmt.Print(" \033[1;32mBright Green" + reset)
	fmt.Print(" \033[1;33mBright Yellow" + reset)
	fmt.Print(" \033[1;36mBright Cyan" + reset)
	fmt.Println(" \033[1;35mBright Magenta" + reset)

	// Line 5: Complex combinations
	fmt.Print("\033[1;37;44mBold White on Blue" + reset)
	fmt.Print(" \033[30;103mBlack on Bright Yellow" + reset)
	fmt.Print(" \033[97;41mBright White on Red" + reset)
	fmt.Println(" \033[1;33;46mBold Yellow on Cyan" + reset)

	// Line 6: More combinations
	fmt.Print("\033[35;47mMagenta on White" + reset)
	fmt.Print(" \033[34;43mBlue on Yellow" + reset)
	fmt.Print(" \033[37;42mWhite on Green" + reset)
	fmt.Print(" \033[31;46mRed on Cyan" + reset)
	fmt.Println(" \033[36;41mCyan on Red" + reset)
}
