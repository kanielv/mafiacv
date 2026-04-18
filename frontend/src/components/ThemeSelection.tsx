import { useState } from "react";
import { useDisclosure } from "@mantine/hooks";
import { Modal, Grid, Button, Textarea, Text, Stack } from "@mantine/core";

interface ThemePreset {
  label: string;
  image: string;
}

const PRESETS: ThemePreset[] = [
  { label: "Beebadoobee London Concert", image: "/beebadoobee.png" },
  { label: "Donut Strat Aftermath", image: "/wawa.png" },
  { label: "Donkey Kong Country", image: "/kong.png" },
  { label: "Little Woman", image: "/women.png" },
  { label: "Les Checkers", image: "/checkers.png" },
  { label: "KSI Music Video", image: "/king.png" },
];

interface ThemeSelectionProps {
  value: string;
  onChange: (theme: string) => void;
}

export default function ThemeSelection({ value, onChange }: ThemeSelectionProps) {
  const [opened, { open, close }] = useDisclosure(false);
  const [customOpened, { open: openCustom, close: closeCustom }] = useDisclosure(false);
  const [customText, setCustomText] = useState(value);

  const pickPreset = (label: string) => {
    onChange(label);
    close();
  };

  const saveCustom = () => {
    const trimmed = customText.trim();
    if (trimmed) onChange(trimmed);
    closeCustom();
  };

  return (
    <Stack gap="xs">
      <Button fullWidth size="md" variant="outline" onClick={open}>
        Theme
      </Button>
      <Text size="sm" c="dimmed" ta="center">
        {value ? `Selected: ${value}` : "No theme selected (will use default)"}
      </Text>

      <Modal
        centered
        opened={opened}
        onClose={close}
        size="90%"
        title="Choose a theme"
      >
        <Grid justify="center" align="center" gutter="lg" mb={10}>
          {PRESETS.map((p) => (
            <Grid.Col
              key={p.label}
              span={6}
              style={{ display: "flex", justifyContent: "center" }}
            >
              <Button
                size="xl"
                variant="outline"
                radius={10}
                styles={{
                  root: {
                    borderWidth: "4px",
                    borderColor: "#3E8E7E",
                    height: "150px",
                    width: "285px",
                    padding: 0,
                    overflow: "hidden",
                    position: "relative",
                  },
                }}
                onClick={() => pickPreset(p.label)}
              >
                <img
                  src={p.image}
                  alt={p.label}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    height: "100%",
                    objectFit: "cover",
                  }}
                />
              </Button>
            </Grid.Col>
          ))}
        </Grid>

        <Grid justify="center" mt={10} mb={10}>
          <Button
            size="lg"
            styles={{
              root: {
                backgroundColor: "#3E8E7E",
                color: "#FFFFFF",
                width: "200px",
                borderRadius: "5px",
              },
            }}
            onClick={() => {
              close();
              openCustom();
            }}
          >
            CUSTOMIZE
          </Button>
        </Grid>
      </Modal>

      <Modal
        centered
        title="Custom theme"
        opened={customOpened}
        onClose={closeCustom}
        size="90%"
      >
        <Textarea
          radius="lg"
          placeholder="Describe your own mafia setting..."
          autosize
          variant="filled"
          minRows={6}
          value={customText}
          onChange={(e) => setCustomText(e.currentTarget.value)}
        />
        <Grid justify="center" mt={10}>
          <Button
            size="lg"
            variant="outline"
            radius={10}
            styles={{
              root: {
                borderWidth: "4px",
                borderColor: "#E94560",
                width: "200px",
              },
            }}
            onClick={saveCustom}
          >
            Save
          </Button>
        </Grid>
      </Modal>
    </Stack>
  );
}
