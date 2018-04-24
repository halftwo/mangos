package vbs


type Kind int8

const (
	VBS_INVALID Kind = 0x0

	VBS_TAIL 	= 0x01         // Used to terminate list or dict.

	VBS_LIST        = 0x02

	VBS_DICT        = 0x03

	// RESERVED       0x04
	// RESERVED       0x05
	// RESERVED       0x06

	// DONT USE       0x07
	// DONT USE       ....
	// DONT USE       0x0D

	// RESERVED       0x0E

	VBS_NULL        = 0x0F

	VBS_DESCRIPTOR  = 0x10         // 0001 0xxx

	VBS_BOOL        = 0x18         // 0001 100x 	0=F 1=T

	// DONT USE       0x1A

	VBS_BLOB        = 0x1B

	VBS_DECIMAL     = 0x1C         // 0001 110x 	0=+ 1=-

	VBS_FLOATING    = 0x1E         // 0001 111x 	0=+ 1=-

	VBS_STRING      = 0x20         // 001x xxxx

	VBS_INTEGER     = 0x40  
)

const VBS_DESCRIPTOR_MAX 	= 0x7fff

const VBS_SPECIAL_DESCRIPTOR 	= 0x8000


var kindNames = [...]string{
        "INVALID",      /*  0 */
        "TAIL",         /*  1 */
        "LIST",         /*  2 */
        "DICT",         /*  3 */
        "NULL",         /*  4 */
        "FLOATING",     /*  5 */
        "DECIMAL",      /*  6 */
        "BOOL",         /*  7 */
        "STRING",       /*  8 */
        "INTEGER",      /*  9 */
        "BLOB",         /* 10 */
        "DESCRIPTOR",   /* 11 */
}

var kindIdx = [VBS_INTEGER + 1]uint8{
         0,  1,  2,  3,  0,  0,  0, 0,  0,  0,  0,  0,  0,  0,  0,  4,
        11,  0,  0,  0,  0,  0,  0, 0,  7,  0,  0, 10,  6,  0,  5,  0,
         8,  0,  0,  0,  0,  0,  0, 0,  0,  0,  0,  0,  0,  0,  0,  0,
         0,  0,  0,  0,  0,  0,  0, 0,  0,  0,  0,  0,  0,  0,  0,  0,
         9,
}

func (k Kind) String() string {
	if k >= 0 && k <= VBS_INTEGER {
		return kindNames[kindIdx[k]]
	}
	return kindNames[0]
}


