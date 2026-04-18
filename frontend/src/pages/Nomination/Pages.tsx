import "@mantine/core/styles.css";
import { Group, Stack, Text, ScrollArea, Title } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import '../../index.css';
import VotePlayer from '../../components/votePlayer';
import { useGame } from "../../context/GameContext";

const NOMINATION_SECONDS = 10;

export default function Nomination() {
  const { players, nominationSubmitted, submitNomination } = useGame();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [secondsLeft, setSecondsLeft] = useState(NOMINATION_SECONDS);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    intervalRef.current = setInterval(() => {
      setSecondsLeft(prev => (prev <= 1 ? 0 : prev - 1));
    }, 1000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  const handleNominate = (socketID: string) => {
    if (nominationSubmitted) return;
    setSelectedId(socketID);
    submitNomination(socketID);
  };

  const alivePlayers = players.filter(p => p.isAlive);

  return (
    <div className="bg-mafiaBlack-default min-h-screen items-center p-4">
      <Group justify="center" align="flex-start" gap={50} className="flex flex-col min-h-screen md:flex-row gap-4 w-full">
        <Stack align="center" justify="center" className="w-full md:w-1/2" style={{ maxWidth: 520 }}>
          <Title order={2} style={{ color: "#E94560", letterSpacing: "0.1em" }}>NOMINATION</Title>
          <Text size="md" c="dimmed">Nominate a player to stand trial.</Text>
          <Text size="lg" c="#3E8E7E" fw={700}>
            0:{secondsLeft.toString().padStart(2, "0")}
          </Text>
          {nominationSubmitted && (
            <Text size="sm" c="#3E8E7E">Nomination submitted.</Text>
          )}
        </Stack>
        <Stack mt="sm" className="w-full md:w-1/2 rounded-md" style={{ border: '4px solid #E94560', maxWidth: 520 }}>
          <Group justify="center" align="center">
            <div className="min-w-[100%] w-[100%] bg-mafiaBlack-default p-4">
              <Text mb="sm" size="md" c="white">Nominations:</Text>
              <ScrollArea h={400}>
                {alivePlayers.map(p => (
                  <VotePlayer
                    key={p.socketID}
                    name={p.name}
                    label={selectedId === p.socketID ? "nominated" : "nominate"}
                    selected={selectedId === p.socketID}
                    onClick={() => handleNominate(p.socketID)}
                  />
                ))}
              </ScrollArea>
            </div>
          </Group>
        </Stack>
      </Group>
    </div>
  );
}
