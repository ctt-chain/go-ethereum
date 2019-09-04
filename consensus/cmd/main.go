package main

import (
	"math/big"
	"fmt"
)

func main() {


	level:= new(big.Int).SetUint64(200000000)
	vote:=new(big.Int).SetInt64(122)
	reward,s:= new(big.Int).SetString("292998504566210000000",10)
	if s{

		v := new(big.Int).Div(level,vote)
		a := new(big.Int).Mul(reward,new(big.Int).SetInt64(9997))

		r := new(big.Int).Mul(v,a)
		c := new(big.Int).Div(r,new(big.Int).SetInt64(10000))
		q := new(big.Int).Div(c,new(big.Int).SetInt64(100000000))
		fmt.Println(v)
		fmt.Println(a)
		fmt.Println(r)
		fmt.Println(q)
	}
	p := new(big.Int).Mul(reward ,new(big.Int).SetInt64(3))
	t := new(big.Int).Div(p ,new(big.Int).SetInt64(10000))

	u,s:= new(big.Int).SetString("4801812428674480895",10)
	if s{
		b := new(big.Int).Mul(u,new(big.Int).SetInt64(61))
		fmt.Println(t)
		fmt.Println(b.Add(b,t))
	}
/*	level:= float64(1)
	vote:=new(big.Float).SetFloat64(10)
	reward,s:= new(big.Float).SetString("2929985045662100")
	if s{

		v := new(big.Float).Quo(new(big.Float).SetFloat64(level),vote)
		a := new(big.Float).Mul(reward,new(big.Float).SetFloat64(0.9997))
		r := new(big.Float).Mul(v,a)
		q := new(big.Float).Mul(r,x)
		if i{
			fmt.Println(v)
			fmt.Println(a)
			fmt.Println(r)
			fmt.Println(q)
		}
	}*/
}