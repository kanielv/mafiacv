import { Navigate } from "react-router-dom";
import { Box, Stack, Title, Text, Loader, Group } from "@mantine/core";
import { useGame } from "../../context/GameContext";
import TypewriterText from "../../components/TypewriterText";

const panelRed = {
  border: "4px solid #E94560",
  borderRadius: "0.5rem",
  backgroundColor: "rgba(29,31,39,0.85)",
} as const;

const panelTeal = {
  border: "4px solid #3E8E7E",
  borderRadius: "0.5rem",
  backgroundColor: "rgba(29,31,39,0.85)",
} as const;

export default function Game() {
  const { role, narration, phase, triggerNightTransition } = useGame();

  if (!role) {
    return <Navigate to="/" replace />;
  }

  if (phase === 'night') {
    return <Navigate to="/night" replace />;
  }

  return (
    <div className="bg-mafiaBlack-default min-h-screen p-4 flex flex-col items-center">
      <Stack gap="lg" style={{ width: "100%", maxWidth: 720 }}>
        <Box p="lg" style={panelRed}>
          <Stack gap={4} align="center">
            <Text size="sm" c="dimmed" tt="uppercase" fw={700}>
              Your Role
            </Text>
            <Title order={1} style={{ color: "#E94560", letterSpacing: "0.1em" }}>
              {role.toUpperCase()}
            </Title>
          </Stack>
        </Box>

        <Box p="lg" style={panelTeal}>
          <Stack gap="sm">
            <Title order={3} style={{ color: "#E94560" }}>
              The Tale Begins
            </Title>
            {narration ? (
              <>
                <TypewriterText
                  text={narration.story}
                  size="lg"
                  style={{ color: "white" }}
                  onDone={triggerNightTransition}
                />
                {phase === 'intro' && (
                  <Text size="sm" c="dimmed" fs="italic" ta="center">
                    The first night falls…
                  </Text>
                )}
              </>
            ) : (
              <Group gap="sm">
                <Loader size="sm" color="#E94560" />
                <Text c="dimmed">
                  The narrator is preparing the tale...
                </Text>
              </Group>
            )}
          </Stack>
        </Box>
      </Stack>
    </div>
  );
}
