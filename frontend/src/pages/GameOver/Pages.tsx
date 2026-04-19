import "@mantine/core/styles.css";
import { Box, Group, Loader, ScrollArea, Stack, Text, Title } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import "../../index.css";
import { useGame } from "../../context/GameContext";
import TypewriterText from "../../components/TypewriterText";

const panelRed = {
  border: "4px solid #E94560",
  borderRadius: "0.5rem",
  backgroundColor: "rgba(29,31,39,0.85)",
} as const;

const COUNTDOWN_SECONDS = 5;
const NARRATION_FALLBACK_MS = 60_000;

export default function GameOver() {
  const { narration, winner, players, resetForHome } = useGame();
  const navigate = useNavigate();

  const [secondsLeft, setSecondsLeft] = useState<number | null>(null);
  const countdownStartedRef = useRef(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const fallbackRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const story =
    narration?.storyType === "game_ending" ? narration.story : "";

  const headline = winner
    ? winner === "mafia"
      ? "The Mafia have taken the town."
      : "The Town has driven out the Mafia."
    : "";

  const startCountdown = () => {
    if (countdownStartedRef.current) return;
    countdownStartedRef.current = true;
    setSecondsLeft(COUNTDOWN_SECONDS);
    intervalRef.current = setInterval(() => {
      setSecondsLeft((prev) => {
        if (prev === null) return prev;
        if (prev <= 1) {
          if (intervalRef.current) clearInterval(intervalRef.current);
          resetForHome();
          navigate("/");
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  };

  useEffect(() => {
    fallbackRef.current = setTimeout(startCountdown, NARRATION_FALLBACK_MS);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
      if (fallbackRef.current) clearTimeout(fallbackRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleNarrationDone = () => {
    if (fallbackRef.current) {
      clearTimeout(fallbackRef.current);
      fallbackRef.current = null;
    }
    startCountdown();
  };

  return (
    <div className="bg-mafiaBlack-default min-h-screen p-4">
      <Stack gap="md" maw={1100} mx="auto">
        <Group justify="space-between" wrap="nowrap">
          <Title order={2} style={{ color: "#E94560", letterSpacing: "0.1em" }}>
            GAME OVER
          </Title>
          {winner && (
            <Text c="#3E8E7E" fw={700} tt="uppercase">
              {winner} wins
            </Text>
          )}
        </Group>

        {headline && (
          <Text size="xl" c="white" fw={700}>
            {headline}
          </Text>
        )}

        <Box p="lg" style={panelRed}>
          <Stack gap="sm">
            <Text size="sm" c="dimmed" tt="uppercase" fw={700}>
              Ending
            </Text>
            <ScrollArea h={360}>
              {story ? (
                <TypewriterText
                  text={story}
                  size="lg"
                  c="white"
                  onDone={handleNarrationDone}
                />
              ) : (
                <Group gap="sm">
                  <Loader size="sm" color="#E94560" />
                  <Text c="dimmed">The narrator closes the book...</Text>
                </Group>
              )}
            </ScrollArea>
          </Stack>
        </Box>

        {players.length > 0 && (
          <Box p="md" style={panelRed}>
            <Stack gap="xs">
              <Text size="sm" c="dimmed" tt="uppercase" fw={700}>
                Final Roster
              </Text>
              {players.map((p) => (
                <Text key={p.socketID} c={p.isAlive ? "white" : "dimmed"}>
                  {p.name} — {p.isAlive ? "alive" : "dead"}
                </Text>
              ))}
            </Stack>
          </Box>
        )}

        {secondsLeft !== null && (
          <Text ta="center" c="dimmed" size="sm">
            Returning to home in {secondsLeft}…
          </Text>
        )}
      </Stack>
    </div>
  );
}
