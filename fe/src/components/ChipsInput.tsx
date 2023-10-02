import { MuiChipsInput } from 'mui-chips-input';
import React, { useEffect, useState, useRef } from 'react';
import { useInput } from 'ra-core';
import { CommonInputProps, ResettableTextFieldProps, sanitizeInputRestProps } from 'react-admin';

export type ChipsInputProps = CommonInputProps & ResettableTextFieldProps;

function ChipsInput(props: ChipsInputProps) {
  const {
    className,
    defaultValue,
    label,
    helperText,
    onChange,
    onBlur,
    onFocus,
    resource,
    source,
    validate,
    sx,
    ...rest
  } = props;

  const {
    id,
    field,
    isRequired,
    // fieldState: { error, invalid, isTouched },
    // formState: { isSubmitted },
  } = useInput({
    defaultValue,
    resource,
    source,
    onBlur,
    onChange,
    type: 'text',
    validate,
    ...rest,
  });

  const [chips, setChips] = useState<string[]>(field.value);
  const hasFocus = useRef(false);

  // update the input text when the record changes
  useEffect(() => {
    if (!hasFocus.current) {
      setChips(field.value);
    }
  }, [field.value]);

  const handleChange = (newChips: string[]) => {
    setChips(newChips);
    field.onChange(newChips);
  };

  const handleFocus = (event: React.FocusEvent<HTMLInputElement>) => {
    if (onFocus) {
      onFocus(event);
    }
    hasFocus.current = true;
  };

  const handleBlur = () => {
    hasFocus.current = false;
    setChips(field.value);
  };

  return (
    <MuiChipsInput
      id={id}
      sx={sx}
      name={field.name}
      label={label}
      onChange={handleChange}
      onFocus={handleFocus}
      onBlur={handleBlur}
      required={isRequired}
      value={chips}
      {...sanitizeInputRestProps(rest)}
    />
  );
}

export default ChipsInput;
