package main

import (
	"errors"
	"math"
	"strconv"
)

var ErrZero = errors.New("zero is not allowed")

func fizzBuzz(n int) (string, error) {
	if n > math.MinInt {
		switch {
		case n == 0:
			return "", ErrZero
		case n%3 == 0 && n%5 == 0:
			return "FizzBuzz", nil
		case n%3 == 0:
			return "Fizz", nil
		case n%5 == 0:
			return "Buzz", nil
		}
	}

	return strconv.Itoa(n), nil
}
