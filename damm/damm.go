/*
 * Damm algorithm
 * https://en.wikipedia.org/wiki/Damm_algorithm
 * http://archiv.ub.uni-marburg.de/diss/z2004/0516/pdf/dhmd.pdf
 * http://www.md-software.de/math/DAMM_Quasigruppen.txt
 */
package damm

func Checksum10(number string) int {
	interim := 0
	for i, r := range number {
		if r >= '0' && r <= '9' {
			interim = int(Table10[interim][r - '0'])
		} else {
			return -(i + 1)
		}
	}
	return interim
}

func Validate10(number string) bool {
	return Checksum10(number) == 0
}

func Checksum16(number string) int {
	interim := 0
	for i, r := range number {
		var k int32
		if r >= '0' && r <= '9' {
			k = r - '0'
		} else if r >= 'A' && r <= 'F' {
			k = r - 'A' + 10
		} else if r >= 'a' && r <= 'f' {
			k = r - 'a' + 10
		} else {
			return -(i + 1)
		}
		interim = int(Table16[interim][k])
	}
	return interim
}

func Validate16(number string) bool {
	return Checksum16(number) == 0
}

