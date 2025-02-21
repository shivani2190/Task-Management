import axios from "axios";

const API_URL = process.env.NEXT_PUBLIC_API_URL;

export async function login(username: string, password: string) {
  try {
    const res = await axios.post(`${API_URL}/login`, { username, password });
    localStorage.setItem("token", res.data.token);
    return true;
  } catch (error) {
    alert("Login failed");
    return false;
  }
}
