package ledsim

import (
	"log"
	"runtime/debug"
	"sort"
	"time"
)

const (
	bucketSize = time.Second
)

type Keyframe struct {
	Label    string
	Offset   time.Duration
	Duration time.Duration
	Effect   Effect
	Layer    int
}

func (k *Keyframe) EndOffset() time.Duration {
	return k.Offset + k.Duration
}

type EffectsManager struct {
	keyframeBuckets [][]*Keyframe // each bucket is 1 second
	lastKeyframes   []*Keyframe
	blacklist       map[*Keyframe]bool
}

func NewEffectsManager(keyframes []*Keyframe) *EffectsManager {
	var keyframeBuckets [][]*Keyframe

	for {
		var bucket []*Keyframe
		lower := time.Duration(len(keyframeBuckets)) * bucketSize
		upper := time.Duration(len(keyframeBuckets)+1) * bucketSize

		outOfBounds := true
		// bucketizing algorithm runs in O(n^2), could optimize to run faster.
		for _, keyframe := range keyframes {
			if (keyframe.Offset < lower && keyframe.EndOffset() > lower) ||
				(keyframe.Offset >= lower && keyframe.Offset < upper) {
				bucket = append(bucket, keyframe)
				outOfBounds = false
			} else if keyframe.Offset > upper {
				outOfBounds = false
			}
		}

		if outOfBounds {
			break
		}

		sort.Slice(bucket, func(i, j int) bool {
			return bucket[i].Layer < bucket[j].Layer
		})

		keyframeBuckets = append(keyframeBuckets, bucket)
		// bucketN := len(keyframeBuckets) - 1
		// fmt.Println("keyframe bucket:", bucketN)
		// for _, keyframe := range bucket {
		// 	fmt.Println(keyframe.Label)
		// }
		// fmt.Println("")

	}

	return &EffectsManager{
		keyframeBuckets: keyframeBuckets,
		lastKeyframes:   []*Keyframe{},
		blacklist:       make(map[*Keyframe]bool),
	}
}

func isKeyframeIn(needle *Keyframe, haystack []*Keyframe) bool {
	for _, keyframe := range haystack {
		if needle == keyframe {
			return true
		}
	}
	return false
}

func (r *EffectsManager) Evaluate(system *System, delta time.Duration) {
	bucketNum := int(delta / bucketSize)
	if bucketNum >= len(r.keyframeBuckets) {
		return
	}

	bucket := r.keyframeBuckets[bucketNum]

	currentKeyframes := make([]*Keyframe, 0, len(bucket))

	for _, keyframe := range bucket {
		if delta >= keyframe.Offset && delta < keyframe.EndOffset() && !r.blacklist[keyframe] {
			currentKeyframes = append(currentKeyframes, keyframe)
		}
	}

	for _, lastKeyframe := range r.lastKeyframes {
		if !isKeyframeIn(lastKeyframe, currentKeyframes) && !r.blacklist[lastKeyframe] {
			// exiting keyframe
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						// get stack trace
						log.Printf("warn: panic OnExit with effect %q: %v\n%s",
							lastKeyframe.Label, rec, string(debug.Stack()))
					}
				}()
				lastKeyframe.Effect.OnExit(system)
			}()
		}
	}

	for _, keyframe := range currentKeyframes {
		if r.blacklist[keyframe] {
			continue
		}

		if !isKeyframeIn(keyframe, r.lastKeyframes) {
			// entering keyframe
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						log.Printf("warn: panic OnEnter with effect %q: %v\n%s",
							keyframe.Label, rec, string(debug.Stack()))
						log.Printf("warn: %q will be blacklisted", keyframe.Label)
						r.blacklist[keyframe] = true
					}
				}()

				keyframe.Effect.OnEnter(system)
			}()
		}
	}

	r.lastKeyframes = currentKeyframes

	for _, keyframe := range currentKeyframes {
		if r.blacklist[keyframe] {
			continue
		}

		progress := float64(delta-keyframe.Offset) / float64(keyframe.Duration)

		func() {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("warn: panic Eval with effect %q: %v\n%s",
						keyframe.Label, rec, string(debug.Stack()))
					log.Printf("warn: %q will be blacklisted", keyframe.Label)
					r.blacklist[keyframe] = true
				}
			}()

			keyframe.Effect.Eval(progress, system)
		}()
	}
}

type EffectsRunner struct {
	manager    *EffectsManager
	start      time.Time
	timeGetter func() (time.Duration, error)
}

func NewEffectsRunner(manager *EffectsManager, timeGetter ...func() (time.Duration, error)) *EffectsRunner {
	if len(timeGetter) > 0 {
		return &EffectsRunner{
			manager:    manager,
			timeGetter: timeGetter[0],
			start:      time.Now(),
		}
	}

	return &EffectsRunner{
		manager: manager,
		start:   time.Now(),
	}
}

func (e *EffectsRunner) Execute(system *System, next func() error) error {
	if e.timeGetter != nil {
		t, err := e.timeGetter()
		if err != nil {
			log.Println("warn: error getting time, falling back to wall clock:", err)
			e.manager.Evaluate(system, time.Since(e.start))
		} else {
			e.start = time.Now().Add(-t)
			e.manager.Evaluate(system, t)
		}

		return next()
	}

	e.manager.Evaluate(system, time.Since(e.start))
	return next()
}
