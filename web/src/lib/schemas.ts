import { z } from 'zod'

export const configSettingSchema = z.object({
  key: z.string().min(1, 'Setting key is required'),
  value: z.string(),
  scope: z.enum(['global', 'project']),
})

export type ConfigSettingForm = z.infer<typeof configSettingSchema>

export const taskStartSchema = z.object({
  source: z.string().min(1, 'Task source is required'),
  description: z.string().optional(),
  tags: z.string().optional(), // comma-separated
})

export type TaskStartForm = z.infer<typeof taskStartSchema>
