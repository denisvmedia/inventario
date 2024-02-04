import {
  ArrayInput,
  AutocompleteInput,
  BooleanInput,
  DateInput,
  FileField,
  FileInput,
  ImageField,
  NumberInput,
  ReferenceInput,
  required,
  SelectInput,
  SimpleForm,
  SimpleFormIterator,
  TextInput,
} from 'react-admin';
import React from 'react';
import { useRecordContext } from 'ra-core';
import { COMMODITY_TYPES_ELEMENTS } from '../api/types/commodity-types';
import { CURRENCIES_ELEMENTS } from '../api/types/currencies';
import { COMMODITY_STATUS_ELEMENTS, COMMODITY_STATUS_IN_USE } from '../api/types/commodity-statuses';
import ChipsInput from './ChipsInput';

function Images(props: any) {
  const record = useRecordContext(props);
  // TODO(2024-02-04): continue here with the images
  // TODO: maybe do the transformation in dataProvider => this will allow to not think about the domain here
  // TODO: display small images, allow deleting them, allow viewing them in a modal
  // TODO(2024-02-04): do the same for manuals and invoices
  const fields = record._meta.images.map((image: any, index: any) => (
    <img src={`http://localhost:3333/api/v1/commodities/${record.id}/images/${image}.png`} />
  ));

  return (
    <div>
      {fields}
    </div>
  );
}

function ComodityBaseForm() {
  return (
    <SimpleForm>
      <TextInput
        source="name"
        validate={[required()]}
        label="Name"
        fullWidth
        name="name"
      />

      <TextInput
        source="short_name"
        validate={[required()]}
        label="Short Name"
        fullWidth
        name="short_name"
      />

      <SelectInput
        source="type"
        validate={[required()]}
        label="type"
        name="type"
        fullWidth
        choices={COMMODITY_TYPES_ELEMENTS}
      />

      <ReferenceInput
        source="area_id"
        validate={[required()]}
        reference="areas"
        name="area_id"
      >
        <AutocompleteInput
          name="area_id"
          validate={[required()]}
          fullWidth
        />
      </ReferenceInput>

      <NumberInput
        source="count"
        validate={[required()]}
        label="Count"
        name="count"
        fullWidth
        step={1}
        min={1}
        defaultValue={1}
      />

      <NumberInput
        source="original_price"
        validate={[required()]}
        label="Original Price"
        name="original_price"
        fullWidth
        step={0.01}
        min={0}
        defaultValue={0}
      />

      <SelectInput
        source="original_price_currency"
        validate={[required()]}
        label="Original Price Currency"
        name="original_price_currency"
        fullWidth
        choices={CURRENCIES_ELEMENTS}
        defaultValue="CZK"
      />

      <NumberInput
        source="converted_original_price"
        validate={[required()]}
        label="Converted Original Price"
        name="converted_original_price"
        fullWidth
        step={0.01}
        min={0}
        defaultValue={0}
      />

      <NumberInput
        source="current_price"
        validate={[required()]}
        label="Current Price"
        name="current_price"
        fullWidth
        step={0.01}
        min={0}
        defaultValue={0}
      />

      <TextInput
        source="serial_number"
        label="Serial Number"
        name="serial_number"
        fullWidth
      />

      <ArrayInput
        source="extra_serial_numbers"
        name="extra_serial_numbers"
        label="Extra Serial Numbers"
      >
        <SimpleFormIterator inline>
          <TextInput source="" />
        </SimpleFormIterator>
      </ArrayInput>

      <ArrayInput
        source="part_numbers"
        name="part_numbers"
        label="Part Numbers"
      >
        <SimpleFormIterator inline>
          <TextInput source="" />
        </SimpleFormIterator>
      </ArrayInput>

      <ChipsInput
        source="tags"
        name="tags"
        label="Tags"
        fullWidth
      />

      <SelectInput
        source="status"
        validate={[required()]}
        label="status"
        name="status"
        fullWidth
        choices={COMMODITY_STATUS_ELEMENTS}
        defaultValue={COMMODITY_STATUS_IN_USE}
      />

      <DateInput
        source="purchase_date"
        validate={[required()]}
        label="Purchase Date"
        name="purchase_date"
        fullWidth
        defaultValue={new Date().toISOString().slice(0, 10)}
      />

      <ArrayInput
        source="urls"
        name="urls"
        label="URLs"
      >
        <SimpleFormIterator inline>
          <TextInput source="" />
        </SimpleFormIterator>
      </ArrayInput>

      <TextInput
        source="comments"
        multiline
        label="Comments"
        name="comments"
        fullWidth
      />

      <Images source="_meta" />

      <FileInput name="images" source="images" multiple>
        <FileField source="src" title="title" />
      </FileInput>

      <FileInput name="manuals" source="manuals" multiple>
        <FileField source="src" title="title" />
      </FileInput>

      <FileInput name="invoices" source="invoices" multiple>
        <FileField source="src" title="title" />
      </FileInput>

      <BooleanInput
        source="draft"
        label="Draft"
        name="draft"
        defaultValue={false}
      />
    </SimpleForm>
  );
}

export default ComodityBaseForm;
