package matching

import "testing"

func TestEngine(t *testing.T) {
	t.Run("empty book creates no trade", func(t *testing.T) { 
		t.Skip("TODO")
	})

	t.Run("full match", func(t *testing.T) { 
		t.Skip("TODO")
	})

	t.Run("no match because of price", func(t *testing.T) { 
		t.Skip("TODO")
	})

	t.Run("partial fill of incoming order", func(t *testing.T) { 
		t.Skip("TODO")
	})

	t.Run("partial fill of resting order", func(t *testing.T) { 
		t.Skip("TODO")
	})

	t.Run("multiple fills", func(t *testing.T) { 
		t.Skip("TODO")
	})

	t.Run("price priority", func(t *testing.T) { 
		t.Skip("TODO")
	})

	t.Run("time priority", func(t *testing.T) { 
		t.Skip("TODO")
	})

	//BUY 10.10 | SELL 10.00
	t.Run("trade price uses resting order price", func(t *testing.T) { 
		t.Skip("TODO")
	})

}
