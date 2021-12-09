package kcp

const maxAutoTuneSamples = 258

type pulse struct {
	bit bool
	seq uint32
}

type autoTune struct {
	pulses [maxAutoTuneSamples]pulse
}

func (tune *autoTune) Sample(bit bool, seq uint32) {
	tune.pulses[seq%maxAutoTuneSamples] = pulse{bit, seq}
}

func (tune *autoTune) FindPeriod(bit bool) int {
	lastPulse := tune.pulses[0]
	idx := 1

	var leftEdge int

	for ; idx < len(tune.pulses); idx++ {
		if lastPulse.bit != bit && tune.pulses[idx].bit == bit {
			if lastPulse.seq+1 == tune.pulses[idx].seq {
				leftEdge = idx

				break
			}
		}

		lastPulse = tune.pulses[idx]
	}

	var rightEdge int

	lastPulse = tune.pulses[leftEdge]
	idx = leftEdge + 1

	for ; idx < len(tune.pulses); idx++ {
		if lastPulse.seq+1 == tune.pulses[idx].seq {
			if lastPulse.bit == bit && tune.pulses[idx].bit != bit {
				rightEdge = idx

				break
			}
		} else {
			return -1
		}

		lastPulse = tune.pulses[idx]
	}

	return rightEdge - leftEdge
}
