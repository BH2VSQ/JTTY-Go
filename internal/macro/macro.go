package macro

import "strings"

type Context struct {
	MyCall    string
	MyGrid    string
	DXCall    string
	Exchange  string
	QueueCall string
}

func Render(template string, c Context) string {
	replacer := strings.NewReplacer(
		"%M", c.MyCall,
		"%G", c.MyGrid,
		"%H", c.DXCall,
		"%E", c.Exchange,
		"%Q", c.QueueCall,
	)
	return replacer.Replace(template)
}
