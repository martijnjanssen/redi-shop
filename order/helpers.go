package order

import (
	"fmt"
	"strconv"
	"strings"
)

func itemStringToJSONString(itemString string) string {
	items := itemStringToMap(itemString)
	itemsString := ""
	for k := range items {
		itemString = fmt.Sprintf("%s\"%s\",", itemString, k)
	}

	return itemsString[:len(itemsString)-1]
}

func itemStringToMap(itemString string) map[string]int {
	m := map[string]int{}

	if itemString == "[]" {
		return m
	}

	items := strings.Split(itemString[1:len(itemString)-1], ",")
	for i := range items {
		item := strings.Split(items[i], "->")
		val, err := strconv.Atoi(item[1])
		if err != nil {
			panic(fmt.Sprintf("invalid string representation of item, %s", items[i]))
		}
		m[item[0]] = val
	}

	return m
}

func mapToItemString(items map[string]int) string {
	s := ""

	for k, v := range items {
		s = fmt.Sprintf("%s%s->%d,", s, k, v)
	}

	if s == "" {
		return "[]"
	}

	return fmt.Sprintf("[%s]", s[:len(s)-1])
}
