import { MantineProvider } from "@mantine/core";
import { theme } from "./theme";
import "./index.css";
import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import Home from "./pages/Home/Pages";
import About from "./pages/About/Pages";
import Michelle from "./pages/MHome/Pages";
import Rules from "./pages/Rules/Pages";
import Lobby from "./pages/Lobby/Pages";
import Night from "./pages/Night/Night";
import Nomination from "./pages/Nomination/Pages";
import Defense from "./pages/Defense/Pages";
import Vote from "./pages/Vote/Pages";
import VoteRecap from "./pages/VoteRecap/Pages";
import Day from "./pages/DayText/Pages";
import Game from "./pages/Game/Pages";
import { GameProvider } from "./context/GameContext";

export default function App() {
  return (
    <MantineProvider theme={theme}>
        <Router>
          <GameProvider>
            <Routes>
              <Route path="/" element={<Home />} />
              <Route path="/about" element={<About />} />
              <Route path="/Michelle" element={<Michelle />} />
              <Route path="/Rules" element={<Rules />} />
              <Route path="/lobby" element={<Lobby />} />
              <Route path="/night" element={<Night />} />
              <Route path="/Nomination" element={<Nomination />} />
              <Route path="/Defense" element={<Defense />} />
              <Route path="/Vote" element={<Vote />} />
              <Route path="/VoteRecap" element={<VoteRecap />} />
              <Route path="/Day" element={<Day />} />
              <Route path="/game" element={<Game />} />
            </Routes>
          </GameProvider>
        </Router>
    </MantineProvider>
  );
}
