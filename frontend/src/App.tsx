import { Route, Routes } from 'react-router'
import OrderPage from './pages/OrderPage'

function App() {
  return (
    <Routes>
      <Route path="/order" element={<OrderPage />} />
      <Route path="/" element={<OrderPage />} />
    </Routes>
  )
}

export default App
