package ircd

var nextIDc chan int64

func init() {
	nextIDc = make(chan int64)
	go func() {
		for i := int64(0); ; i++ {
			nextIDc <- i
		}
	}()
}

func nextID() int64 {
	return <-nextIDc
}
