import { useLocation, Navigate } from "react-router-dom";

export default function Game() {
  const location = useLocation();
  const role = (location.state as { role?: string })?.role;

  if (!role) {
    return <Navigate to="/" replace />;
  }

  return (
    <div style={{ padding: "20px" }}>
      <h1>Your Role: {role}</h1>
    </div>
  );
}
