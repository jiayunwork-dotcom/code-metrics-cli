package utils

import "fmt"

const (
	ColorReset         = "\033[0m"
	ColorBlack         = "\033[30m"
	ColorRed           = "\033[31m"
	ColorGreen         = "\033[32m"
	ColorYellow        = "\033[33m"
	ColorBlue          = "\033[34m"
	ColorMagenta       = "\033[35m"
	ColorCyan          = "\033[36m"
	ColorWhite         = "\033[37m"
	ColorBrightBlack   = "\033[90m"
	ColorBrightRed     = "\033[91m"
	ColorBrightGreen   = "\033[92m"
	ColorBrightYellow  = "\033[93m"
	ColorBrightBlue    = "\033[94m"
	ColorBrightMagenta = "\033[95m"
	ColorBrightCyan    = "\033[96m"
	ColorBrightWhite   = "\033[97m"

	ColorBold = "\033[1m"
)

func C(s, color string) string {
	return color + s + ColorReset
}

func Bold(s string) string {
	return ColorBold + s + ColorReset
}

func Red(s string) string {
	return C(s, ColorBrightRed)
}

func Green(s string) string {
	return C(s, ColorBrightGreen)
}

func Yellow(s string) string {
	return C(s, ColorBrightYellow)
}

func Blue(s string) string {
	return C(s, ColorBrightBlue)
}

func Magenta(s string) string {
	return C(s, ColorBrightMagenta)
}

func Cyan(s string) string {
	return C(s, ColorBrightCyan)
}

func White(s string) string {
	return C(s, ColorBrightWhite)
}

func BrightRed(s string) string {
	return C(s, ColorBrightRed)
}

func BrightGreen(s string) string {
	return C(s, ColorBrightGreen)
}

func BrightYellow(s string) string {
	return C(s, ColorBrightYellow)
}

func BrightBlue(s string) string {
	return C(s, ColorBrightBlue)
}

func BrightMagenta(s string) string {
	return C(s, ColorBrightMagenta)
}

func BrightCyan(s string) string {
	return C(s, ColorBrightCyan)
}

func GradeColor(grade string) string {
	switch grade {
	case "A":
		return ColorBrightGreen
	case "B":
		return ColorBrightCyan
	case "C":
		return ColorBrightYellow
	case "D":
		return ColorBrightMagenta
	case "F":
		return ColorBrightRed
	default:
		return ColorBrightWhite
	}
}

func GetGradeColor(grade string) string {
	return GradeColor(grade)
}

func FormatSprintf(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a...)
}
