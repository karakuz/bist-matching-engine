import { Navigate, Route, Routes } from 'react-router'
import OrderPage from './pages/OrderPage'

function App() {
  return (
    <Routes>
      <Route path="/order" element={<OrderPage />} />
      <Route
        path="/"
        element={<Navigate to="/order?symbol=ASELS" replace />}
      />
    </Routes>
  )
}

export default App
