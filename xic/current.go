package xic

type _Current struct {
	_InQuest
	con *_Connection
}

func newCurrent(con *_Connection, q *_InQuest) *_Current {
	return &_Current{_InQuest: *q, con: con}
}

func (cur *_Current) Txid() int64	{ return cur.txid }
func (cur *_Current) Service() string	{ return cur.service }
func (cur *_Current) Method() string	{ return cur.method }
func (cur *_Current) Ctx() Context	{ return cur.ctx }
func (cur *_Current) Con() Connection	{ return cur.con }

