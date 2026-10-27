<script setup lang="ts">
import { label } from "../api";
defineProps<{ status: string; text?: string }>();
const classes: Record<string, string> = {
  FOUND: "found",
  CLAIM_PENDING: "pending",
  CLAIMED: "claimed",
  RETURNED: "returned",
  CLOSED: "closed",
  PENDING: "pending",
  APPROVED: "claimed",
  REJECTED: "rejected",
  WITHDRAWN: "closed",
  SUPERSEDED: "closed",
  CANCELLED: "closed",
  REVOKED: "rejected",
  PUBLIC: "found",
  HIDDEN: "closed",
};
</script>
<template>
  <span class="tag" :class="'tag-' + (classes[status] || 'closed')">{{
    text || label(status)
  }}</span>
</template>
