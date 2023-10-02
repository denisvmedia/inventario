import React from 'react';
import { MuiChipsInput } from 'mui-chips-input';
import PropTypes from 'prop-types';
import { useInput } from 'ra-core';
import { CommonInputProps, ResettableTextFieldProps, sanitizeInputRestProps } from 'react-admin';

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
    value,
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

  const [chips, setChips] = React.useState('');

  const hasFocus = React.useRef(false);

  // update the input text when the record changes
  React.useEffect(() => {
    if (!hasFocus.current) {
      setChips(field.value);
    }
  }, [field.value]);

  const handleChange = (newChips: string) => {
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

ChipsInput.propTypes = {
  className: PropTypes.string,
  label: PropTypes.oneOfType([
    PropTypes.string,
    PropTypes.bool,
    PropTypes.element,
  ]),
  resource: PropTypes.string,
  source: PropTypes.string,
};

export type ChipsInputProps = CommonInputProps &
Omit<ResettableTextFieldProps, 'label' | 'helperText' | 'value'> & {
  value: string[];
};

ChipsInput.defaultProps = {
  className: '',
  label: '',
  resource: '',
  source: '',
};

export default ChipsInput;
