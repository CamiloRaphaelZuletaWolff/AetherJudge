"use client";

import { Info } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { useProfileStore } from "@/stores/profile";

// Editing works against local state today (no profile API yet). The form is
// wired so swapping in a mutation later is a one-line change.
export function SettingsForm() {
  const { displayName, bio, website, setPrefs } = useProfileStore();
  const { toast } = useToast();

  const [form, setForm] = useState({ displayName, bio, website });
  const dirty = form.displayName !== displayName || form.bio !== bio || form.website !== website;

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        setPrefs(form);
        toast("Profile saved");
      }}
      className="flex flex-col gap-5"
    >
      <Card className="flex items-start gap-2.5 border-v-ce/30 bg-v-ce/10 p-3 text-sm text-v-ce">
        <Info className="mt-0.5 size-4 shrink-0" />
        <p>
          These preferences are saved locally on this device for now. They&apos;ll sync to your
          account once the profile API ships.
        </p>
      </Card>

      <Field
        label="Display name"
        value={form.displayName}
        onChange={(e) => setForm((f) => ({ ...f, displayName: e.target.value }))}
        placeholder="How your name appears to others"
      />
      <Field
        label="Website"
        value={form.website}
        onChange={(e) => setForm((f) => ({ ...f, website: e.target.value }))}
        placeholder="https://your-site.dev"
      />
      <div className="flex flex-col gap-1.5">
        <label htmlFor="bio" className="text-sm font-medium text-foreground">
          Bio
        </label>
        <Textarea
          id="bio"
          rows={4}
          value={form.bio}
          onChange={(e) => setForm((f) => ({ ...f, bio: e.target.value }))}
          placeholder="A line or two about you"
          className="font-sans"
        />
      </div>

      <div className="flex justify-end">
        <Button type="submit" disabled={!dirty}>
          Save changes
        </Button>
      </div>
    </form>
  );
}
