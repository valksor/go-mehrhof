import { cloneElement, createContext, isValidElement, useContext, useId } from 'react'
import type { ComponentProps, ReactNode } from 'react'
import type { ControllerProps, FieldPath, FieldValues } from 'react-hook-form'
import { Controller, FormProvider, useFormContext, type UseFormReturn } from 'react-hook-form'

// --- Form (wraps FormProvider) ---

function Form<T extends FieldValues>({
  children,
  ...form
}: UseFormReturn<T> & { children: ReactNode }) {
  return <FormProvider {...form}>{children}</FormProvider>
}

// --- FormField (wraps Controller with field name context) ---

type FormFieldContextValue = {
  name: string
}

const FormFieldContext = createContext<FormFieldContextValue>(
  {} as FormFieldContextValue,
)

function FormField<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(props: ControllerProps<TFieldValues, TName>) {
  return (
    <FormFieldContext.Provider value={{ name: props.name }}>
      <Controller {...props} />
    </FormFieldContext.Provider>
  )
}

// --- useFormField hook ---

type FormItemContextValue = {
  id: string
}

const FormItemContext = createContext<FormItemContextValue>(
  {} as FormItemContextValue,
)

function useFormField() {
  const fieldContext = useContext(FormFieldContext)
  const itemContext = useContext(FormItemContext)

  if (!fieldContext.name) {
    throw new Error('useFormField must be used within <FormField>')
  }

  if (!itemContext.id) {
    throw new Error('useFormField must be used within <FormItem>')
  }

  const { getFieldState, formState } = useFormContext()
  const fieldState = getFieldState(fieldContext.name, formState)
  const { id } = itemContext

  return {
    id,
    name: fieldContext.name,
    formItemId: `${id}-form-item`,
    formDescriptionId: `${id}-form-item-description`,
    formMessageId: `${id}-form-item-message`,
    ...fieldState,
  }
}

// --- FormItem (layout wrapper with DaisyUI form-control) ---

function FormItem({ className, ...props }: ComponentProps<'div'>) {
  const id = useId()

  return (
    <FormItemContext.Provider value={{ id }}>
      <div className={`form-control w-full ${className ?? ''}`} {...props} />
    </FormItemContext.Provider>
  )
}

// --- FormLabel (label connected to field) ---

function FormLabel({ className, ...props }: ComponentProps<'label'>) {
  const { error, formItemId } = useFormField()

  return (
    <label
      className={`label ${error ? 'text-error' : ''} ${className ?? ''}`}
      htmlFor={formItemId}
      {...props}
    />
  )
}

// --- FormControl (connects input to field with aria attributes) ---

function FormControl({ children }: { children: ReactNode }) {
  const { error, formItemId, formDescriptionId, formMessageId } =
    useFormField()

  const ariaDescribedBy = error
    ? `${formDescriptionId} ${formMessageId}`
    : formDescriptionId

  if (!isValidElement(children)) {
    return <>{children}</>
  }

  return cloneElement(children, {
    id: formItemId,
    'aria-describedby': ariaDescribedBy,
    'aria-invalid': error ? true : undefined,
  } as Record<string, unknown>)
}

// --- FormDescription (help text) ---

function FormDescription({ className, ...props }: ComponentProps<'p'>) {
  const { formDescriptionId } = useFormField()

  return (
    <p
      id={formDescriptionId}
      className={`text-xs opacity-60 mt-1 ${className ?? ''}`}
      {...props}
    />
  )
}

// --- FormMessage (validation error) ---

function FormMessage({
  className,
  children,
  ...props
}: ComponentProps<'p'>) {
  const { error, formMessageId } = useFormField()
  const body = error?.message ? String(error.message) : children

  if (!body) {
    return null
  }

  return (
    <p
      id={formMessageId}
      className={`text-xs text-error mt-1 ${className ?? ''}`}
      role="alert"
      {...props}
    >
      {body}
    </p>
  )
}

export {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormDescription,
  FormMessage,
  useFormField,
}
