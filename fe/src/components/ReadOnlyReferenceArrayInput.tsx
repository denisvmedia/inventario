import React, { useEffect, useState } from 'react';
import { useRecordContext, ChipField, useDataProvider } from 'react-admin';
import Chip from '@mui/material/Chip';
import TextField from '@mui/material/TextField';
import InputAdornment from '@mui/material/InputAdornment';

function ReadOnlyReferenceArrayInput({ source, reference, label }: any) {
  const record = useRecordContext();
  const selectedIds = record[source] || [];
  const [selectedRecords, setSelectedRecords] = useState([]);
  const dataProvider = useDataProvider();

  useEffect(() => {
    if (selectedIds.length > 0) {
      // Fetch the related records based on the selectedIds
      dataProvider
        .getMany(reference, {
          ids: selectedIds,
        })
        .then(({ data }: any) => setSelectedRecords(data));
    }
  }, [dataProvider, reference, selectedIds]);

  return (
    <TextField
      variant="filled"
      label={label}
      sx={{ margin: '3px' }}
      InputProps={{
        readOnly: false,
        fullWidth: true,
        sx: { paddingLeft: '6px', paddingTop: '19px' },
        startAdornment: (
          <InputAdornment
            position="start"
            sx={{
              flexWrap: 'wrap',
              marginTop: '1px !important',
              height: 'inherit',
              maxHeight: 'inherit',
            }}
          >
            {selectedRecords.map((record: any) => (
              <Chip
                key={record.id}
                label={record.name}
                variant="filled"
                size="small"
                sx={{ margin: '3px' }}
              />
            ))}
          </InputAdornment>
        ),
        endAdornment: null,
      }}
    />
  );
}

export default ReadOnlyReferenceArrayInput;
